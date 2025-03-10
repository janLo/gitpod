/**
 * Copyright (c) 2020 Gitpod GmbH. All rights reserved.
 * Licensed under the Gitpod Enterprise Source Code License,
 * See License.enterprise.txt in the project root folder.
 */

import { DBWithTracing, TracedWorkspaceDB, WorkspaceDB } from "@gitpod/gitpod-db/lib";
import {
    CommitContext,
    CommitInfo,
    PrebuiltWorkspace,
    Project,
    ProjectEnvVar,
    StartPrebuildContext,
    StartPrebuildResult,
    TaskConfig,
    User,
    Workspace,
    WorkspaceConfig,
    WorkspaceInstance,
} from "@gitpod/gitpod-protocol";
import { log } from "@gitpod/gitpod-protocol/lib/util/logging";
import { TraceContext } from "@gitpod/gitpod-protocol/lib/util/tracing";
import { getCommitInfo, HostContextProvider } from "../../../src/auth/host-context-provider";
import { WorkspaceFactory } from "../../../src/workspace/workspace-factory";
import { ConfigProvider } from "../../../src/workspace/config-provider";
import { WorkspaceStarter } from "../../../src/workspace/workspace-starter";
import { Config } from "../../../src/config";
import { ProjectsService } from "../../../src/projects/projects-service";
import { secondsBefore } from "@gitpod/gitpod-protocol/lib/util/timeutil";

import { inject, injectable } from "inversify";
import * as opentracing from "opentracing";
import { StopWorkspacePolicy } from "@gitpod/ws-manager/lib";
import { error } from "console";

export class WorkspaceRunningError extends Error {
    constructor(msg: string, public instance: WorkspaceInstance) {
        super(msg);
    }
}

export interface StartPrebuildParams {
    user: User;
    context: CommitContext;
    project?: Project;
    commitInfo?: CommitInfo;
}

const PREBUILD_LIMITER_WINDOW_SECONDS = 60;
const PREBUILD_LIMITER_DEFAULT_LIMIT = 50;

@injectable()
export class PrebuildManager {
    @inject(TracedWorkspaceDB) protected readonly workspaceDB: DBWithTracing<WorkspaceDB>;
    @inject(WorkspaceFactory) protected readonly workspaceFactory: WorkspaceFactory;
    @inject(WorkspaceStarter) protected readonly workspaceStarter: WorkspaceStarter;
    @inject(HostContextProvider) protected readonly hostContextProvider: HostContextProvider;
    @inject(ConfigProvider) protected readonly configProvider: ConfigProvider;
    @inject(Config) protected readonly config: Config;
    @inject(ProjectsService) protected readonly projectService: ProjectsService;

    async abortPrebuildsForBranch(ctx: TraceContext, project: Project, user: User, branch: string): Promise<void> {
        const span = TraceContext.startSpan("abortPrebuildsForBranch", ctx);
        try {
            const prebuilds = await this.workspaceDB
                .trace({ span })
                .findActivePrebuiltWorkspacesByBranch(project.id, branch);
            const results: Promise<any>[] = [];
            for (const prebuild of prebuilds) {
                try {
                    for (const instance of prebuild.instances) {
                        log.info(
                            { userId: user.id, instanceId: instance.id, workspaceId: instance.workspaceId },
                            "Aborting Prebuild workspace because a newer commit was pushed to the same branch.",
                        );
                        results.push(
                            this.workspaceStarter.stopWorkspaceInstance(
                                { span },
                                instance.id,
                                instance.region,
                                StopWorkspacePolicy.ABORT,
                            ),
                        );
                    }
                    prebuild.prebuild.state = "aborted";
                    prebuild.prebuild.error = "A newer commit was pushed to the same branch.";
                    results.push(this.workspaceDB.trace({ span }).storePrebuiltWorkspace(prebuild.prebuild));
                } catch (err) {
                    error("Cannot cancel prebuild", { prebuildID: prebuild.prebuild.id }, err);
                }
            }
            await Promise.all(results);
        } finally {
            span.finish();
        }
    }

    async startPrebuild(
        ctx: TraceContext,
        { context, project, user, commitInfo }: StartPrebuildParams,
    ): Promise<StartPrebuildResult> {
        const span = TraceContext.startSpan("startPrebuild", ctx);
        const cloneURL = context.repository.cloneUrl;
        const commitSHAIdentifier = CommitContext.computeHash(context);
        span.setTag("cloneURL", cloneURL);
        span.setTag("commit", commitInfo?.sha);

        try {
            if (user.blocked) {
                throw new Error(`Blocked users cannot start prebuilds (${user.name})`);
            }
            const existingPB = await this.workspaceDB
                .trace({ span })
                .findPrebuiltWorkspaceByCommit(cloneURL, commitSHAIdentifier);
            // If the existing prebuild is failed, we want to retrigger it.
            if (
                !!existingPB &&
                existingPB.state !== "aborted" &&
                existingPB.state !== "failed" &&
                existingPB.state !== "timeout"
            ) {
                // If the existing prebuild is based on an outdated project config, we also want to retrigger it.
                const existingPBWS = await this.workspaceDB.trace({ span }).findById(existingPB.buildWorkspaceId);
                const existingConfig = existingPBWS?.config;
                const newConfig = await this.fetchConfig({ span }, user, context);
                log.debug(
                    `startPrebuild | commits: ${commitSHAIdentifier}, existingPB: ${
                        existingPB.id
                    }, existingConfig: ${JSON.stringify(existingConfig)}, newConfig: ${JSON.stringify(newConfig)}}`,
                );
                const filterPrebuildTasks = (tasks: TaskConfig[] = []) =>
                    tasks
                        .map((task) =>
                            Object.keys(task)
                                .filter((key) => ["before", "init", "prebuild"].includes(key))
                                // @ts-ignore
                                .reduce((obj, key) => ({ ...obj, [key]: task[key] }), {}),
                        )
                        .filter((task) => Object.keys(task).length > 0);
                const isSameConfig =
                    JSON.stringify(filterPrebuildTasks(existingConfig?.tasks)) ===
                    JSON.stringify(filterPrebuildTasks(newConfig?.tasks));
                // If there is an existing prebuild that isn't failed and it's based on the current config, we return it here instead of triggering a new prebuild.
                if (isSameConfig) {
                    return { prebuildId: existingPB.id, wsid: existingPB.buildWorkspaceId, done: true };
                }
            }
            if (project && context.ref && !project.settings?.keepOutdatedPrebuildsRunning) {
                try {
                    await this.abortPrebuildsForBranch({ span }, project, user, context.ref);
                } catch (e) {
                    console.error("Error aborting prebuilds", e);
                }
            }

            const prebuildContext: StartPrebuildContext = {
                title: `Prebuild of "${context.title}"`,
                actual: context,
                project,
                branch: context.ref,
                normalizedContextURL: context.normalizedContextURL,
            };

            if (this.shouldPrebuildIncrementally(context.repository.cloneUrl, project)) {
                const maxDepth = this.config.incrementalPrebuilds.commitHistory;
                const hostContext = this.hostContextProvider.get(context.repository.host);
                const repoProvider = hostContext?.services?.repositoryProvider;
                if (repoProvider) {
                    prebuildContext.commitHistory = await repoProvider.getCommitHistory(
                        user,
                        context.repository.owner,
                        context.repository.name,
                        context.revision,
                        maxDepth,
                    );
                    if (
                        context.additionalRepositoryCheckoutInfo &&
                        context.additionalRepositoryCheckoutInfo.length > 0
                    ) {
                        const histories = context.additionalRepositoryCheckoutInfo.map(async (info) => {
                            const commitHistory = await repoProvider.getCommitHistory(
                                user,
                                info.repository.owner,
                                info.repository.name,
                                info.revision,
                                maxDepth,
                            );
                            return {
                                cloneUrl: info.repository.cloneUrl,
                                commitHistory,
                            };
                        });
                        prebuildContext.additionalRepositoryCommitHistories = await Promise.all(histories);
                    }
                }
            }

            const projectEnvVarsPromise = project ? this.projectService.getProjectEnvironmentVariables(project.id) : [];

            const workspace = await this.workspaceFactory.createForContext(
                { span },
                user,
                prebuildContext,
                context.normalizedContextURL!,
            );
            const prebuild = await this.workspaceDB.trace({ span }).findPrebuildByWorkspaceID(workspace.id)!;
            if (!prebuild) {
                throw new Error(`Failed to create a prebuild for: ${context.normalizedContextURL}`);
            }

            if (project) {
                let aCommitInfo = commitInfo;
                if (!aCommitInfo) {
                    aCommitInfo = await getCommitInfo(
                        this.hostContextProvider,
                        user,
                        context.repository.cloneUrl,
                        context.revision,
                    );
                    if (!aCommitInfo) {
                        aCommitInfo = {
                            author: "unknown",
                            commitMessage: "unknown",
                            sha: context.revision,
                        };
                    }
                }
                await this.storePrebuildInfo({ span }, project, prebuild, workspace, user, aCommitInfo);
            }

            if (await this.shouldRateLimitPrebuild(span, cloneURL)) {
                prebuild.state = "aborted";
                prebuild.error =
                    "Prebuild is rate limited. Please contact Gitpod if you believe this happened in error.";
                await this.workspaceDB.trace({ span }).storePrebuiltWorkspace(prebuild);
                span.setTag("ratelimited", true);
            } else if (project && (await this.shouldSkipInactiveProject(project))) {
                prebuild.state = "aborted";
                prebuild.error =
                    "Project is inactive. Please start a new workspace for this project to re-enable prebuilds.";
                await this.workspaceDB.trace({ span }).storePrebuiltWorkspace(prebuild);
            } else if (!project && (await this.shouldSkipInactiveRepository({ span }, cloneURL))) {
                prebuild.state = "aborted";
                prebuild.error =
                    "Repository is inactive. Please create a project for this repository to re-enable prebuilds.";
                await this.workspaceDB.trace({ span }).storePrebuiltWorkspace(prebuild);
            } else {
                span.setTag("starting", true);
                const projectEnvVars = await projectEnvVarsPromise;
                const usePVC = this.shouldUsePersistentVolumeClaim(project);
                await this.workspaceStarter.startWorkspace({ span }, workspace, user, [], projectEnvVars, {
                    excludeFeatureFlags: ["full_workspace_backup"],
                    pvcEnabledForPrebuilds: usePVC,
                });
            }

            return { prebuildId: prebuild.id, wsid: workspace.id, done: false };
        } catch (err) {
            TraceContext.setError({ span }, err);
            throw err;
        } finally {
            span.finish();
        }
    }

    async retriggerPrebuild(ctx: TraceContext, user: User, workspaceId: string): Promise<StartPrebuildResult> {
        const span = TraceContext.startSpan("retriggerPrebuild", ctx);
        span.setTag("workspaceId", workspaceId);
        try {
            const workspacePromise = this.workspaceDB.trace({ span }).findById(workspaceId);
            const prebuildPromise = this.workspaceDB.trace({ span }).findPrebuildByWorkspaceID(workspaceId);
            const runningInstance = await this.workspaceDB.trace({ span }).findRunningInstance(workspaceId);
            if (runningInstance !== undefined) {
                throw new WorkspaceRunningError("Workspace is still runnning", runningInstance);
            }
            span.setTag("starting", true);
            const workspace = await workspacePromise;
            if (!workspace) {
                console.error("Unknown workspace id.", { workspaceId });
                throw new Error("Unknown workspace " + workspaceId);
            }
            const prebuild = await prebuildPromise;
            if (!prebuild) {
                throw new Error("No prebuild found for workspace " + workspaceId);
            }
            let projectEnvVars: ProjectEnvVar[] = [];
            if (workspace.projectId) {
                projectEnvVars = await this.projectService.getProjectEnvironmentVariables(workspace.projectId);
            }
            await this.workspaceStarter.startWorkspace({ span }, workspace, user, [], projectEnvVars);
            return { prebuildId: prebuild.id, wsid: workspace.id, done: false };
        } catch (err) {
            TraceContext.setError({ span }, err);
            throw err;
        } finally {
            span.finish();
        }
    }

    shouldPrebuild(config: WorkspaceConfig | undefined): boolean {
        if (!config || !config._origin || config._origin !== "repo") {
            // we demand an explicit gitpod config
            return false;
        }

        const hasPrebuildTask = !!config.tasks && config.tasks.find((t) => !!t.before || !!t.init || !!t.prebuild);
        if (!hasPrebuildTask) {
            return false;
        }

        return true;
    }

    protected shouldPrebuildIncrementally(cloneUrl: string, project?: Project): boolean {
        if (project?.settings?.useIncrementalPrebuilds) {
            return true;
        }
        const trimRepoUrl = (url: string) => url.replace(/\/$/, "").replace(/\.git$/, "");
        const repoUrl = trimRepoUrl(cloneUrl);
        return this.config.incrementalPrebuilds.repositoryPasslist.some((url) => trimRepoUrl(url) === repoUrl);
    }

    protected shouldUsePersistentVolumeClaim(project?: Project): boolean {
        if (project?.settings?.usePersistentVolumeClaim) {
            return true;
        }
        return false;
    }

    async fetchConfig(ctx: TraceContext, user: User, context: CommitContext): Promise<WorkspaceConfig> {
        const span = TraceContext.startSpan("fetchConfig", ctx);
        try {
            return (await this.configProvider.fetchConfig({ span }, user, context)).config;
        } catch (err) {
            TraceContext.setError({ span }, err);
            throw err;
        } finally {
            span.finish();
        }
    }

    //TODO this doesn't belong so deep here. All this context should be stored on the surface not passed down.
    protected async storePrebuildInfo(
        ctx: TraceContext,
        project: Project,
        pws: PrebuiltWorkspace,
        ws: Workspace,
        user: User,
        commit: CommitInfo,
    ) {
        const span = TraceContext.startSpan("storePrebuildInfo", ctx);
        const { userId, teamId, name: projectName, id: projectId } = project;
        try {
            await this.workspaceDB.trace({ span }).storePrebuildInfo({
                id: pws.id,
                buildWorkspaceId: pws.buildWorkspaceId,
                basedOnPrebuildId: ws.basedOnPrebuildId,
                teamId,
                userId,
                projectName,
                projectId,
                startedAt: pws.creationTime,
                startedBy: "", // TODO
                startedByAvatar: "", // TODO
                cloneUrl: pws.cloneURL,
                branch: pws.branch || "unknown",
                changeAuthor: commit.author,
                changeAuthorAvatar: commit.authorAvatarUrl,
                changeDate: commit.authorDate || "",
                changeHash: commit.sha,
                changeTitle: commit.commitMessage,
                // changePR
                changeUrl: ws.contextURL,
            });
        } catch (err) {
            TraceContext.setError(ctx, err);
            throw err;
        } finally {
            span.finish();
        }
    }

    private async shouldRateLimitPrebuild(span: opentracing.Span, cloneURL: string): Promise<boolean> {
        const windowStart = secondsBefore(new Date().toISOString(), PREBUILD_LIMITER_WINDOW_SECONDS);
        const unabortedCount = await this.workspaceDB
            .trace({ span })
            .countUnabortedPrebuildsSince(cloneURL, new Date(windowStart));
        const limit = this.getPrebuildRateLimitForCloneURL(cloneURL);

        if (unabortedCount >= limit) {
            log.debug("Prebuild exceeds rate limit", { limit, unabortedPrebuildsCount: unabortedCount, cloneURL });
            return true;
        }
        return false;
    }

    private getPrebuildRateLimitForCloneURL(cloneURL: string): number {
        // First we use any explicit overrides for a given cloneURL
        let limit = this.config.prebuildLimiter[cloneURL];
        if (limit > 0) {
            return limit;
        }

        // Find if there is a default value set under the '*' key
        limit = this.config.prebuildLimiter["*"];
        if (limit > 0) {
            return limit;
        }

        // Last resort default
        return PREBUILD_LIMITER_DEFAULT_LIMIT;
    }

    private async shouldSkipInactiveProject(project: Project): Promise<boolean> {
        const usage = await this.projectService.getProjectUsage(project.id);
        if (!usage?.lastWorkspaceStart) {
            return false;
        }
        const now = Date.now();
        const lastUse = new Date(usage.lastWorkspaceStart).getTime();
        const inactiveProjectTime = 1000 * 60 * 60 * 24 * 7 * 1; // 1 week
        return now - lastUse > inactiveProjectTime;
    }

    private async shouldSkipInactiveRepository(ctx: TraceContext, cloneURL: string): Promise<boolean> {
        const span = TraceContext.startSpan("shouldSkipInactiveRepository", ctx);
        const { inactivityPeriodForRepos } = this.config;
        if (!inactivityPeriodForRepos) {
            // skipping is disabled if `inactivityPeriodForRepos` is not set
            span.finish();
            return false;
        }
        try {
            return (
                (await this.workspaceDB
                    .trace({ span })
                    .getWorkspaceCountByCloneURL(cloneURL, inactivityPeriodForRepos /* in days */, "regular")) === 0
            );
        } catch (error) {
            log.error("cannot compute activity for repository", { cloneURL }, error);
            TraceContext.setError(ctx, error);
            return false;
        } finally {
            span.finish();
        }
    }
}
