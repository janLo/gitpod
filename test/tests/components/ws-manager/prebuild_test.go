// Copyright (c) 2020 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package wsmanager

import (
	"context"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	csapi "github.com/gitpod-io/gitpod/content-service/api"
	"github.com/gitpod-io/gitpod/test/pkg/integration"
	wsmanapi "github.com/gitpod-io/gitpod/ws-manager/api"
)

func TestPrebuildWorkspaceTaskSuccess(t *testing.T) {
	f := features.New("prebuild").
		WithLabel("component", "ws-manager").
		Assess("it should create a prebuild and succeed the defined tasks", func(_ context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			api := integration.NewComponentAPI(ctx, cfg.Namespace(), kubeconfig, cfg.Client())
			t.Cleanup(func() {
				api.Done(t)
			})

			tests := []struct {
				Name string
				FF   []wsmanapi.WorkspaceFeatureFlag
			}{
				{Name: "classic"},
				{Name: "pvc", FF: []wsmanapi.WorkspaceFeatureFlag{wsmanapi.WorkspaceFeatureFlag_PERSISTENT_VOLUME_CLAIM}},
			}
			for _, test := range tests {
				t.Run(test.Name, func(t *testing.T) {
					_, stopWs, err := integration.LaunchWorkspaceDirectly(t, ctx, api, integration.WithRequestModifier(func(req *wsmanapi.StartWorkspaceRequest) error {
						req.Type = wsmanapi.WorkspaceType_PREBUILD
						req.Spec.Envvars = append(req.Spec.Envvars, &wsmanapi.EnvironmentVariable{
							Name:  "GITPOD_TASKS",
							Value: `[{ "init": "echo \"some output\" > someFile; sleep 20; exit 0;" }]`,
						})
						req.Spec.FeatureFlags = test.FF
						req.Spec.Initializer = &csapi.WorkspaceInitializer{
							Spec: &csapi.WorkspaceInitializer_Git{
								Git: &csapi.GitInitializer{
									RemoteUri:        "https://github.com/gitpod-io/empty.git",
									TargetMode:       csapi.CloneTargetMode_REMOTE_BRANCH,
									CloneTaget:       "main",
									CheckoutLocation: "empty",
									Config:           &csapi.GitConfig{},
								},
							},
						}
						req.Spec.WorkspaceLocation = "empty"
						return nil
					}))
					if err != nil {
						t.Fatalf("cannot launch a workspace: %q", err)
					}
					t.Cleanup(func() {
						sctx, scancel := context.WithTimeout(context.Background(), 5*time.Minute)
						defer scancel()

						sapi := integration.NewComponentAPI(sctx, cfg.Namespace(), kubeconfig, cfg.Client())
						defer sapi.Done(t)

						_, err = stopWs(true, sapi)
						if err != nil {
							t.Errorf("cannot stop workspace: %q", err)
						}
					})
				})
			}
			return ctx
		}).
		Feature()

	testEnv.Test(t, f)
}

func TestPrebuildWorkspaceTaskFail(t *testing.T) {
	t.Skip("status never returns HeadlessTaskFailed (exit 1)")

	f := features.New("prebuild").
		WithLabel("component", "server").
		Assess("it should create a prebuild and fail after running the defined tasks", func(_ context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			api := integration.NewComponentAPI(ctx, cfg.Namespace(), kubeconfig, cfg.Client())
			t.Cleanup(func() {
				api.Done(t)
			})

			ws, stopWs, err := integration.LaunchWorkspaceDirectly(t, ctx, api, integration.WithRequestModifier(func(req *wsmanapi.StartWorkspaceRequest) error {
				req.Type = wsmanapi.WorkspaceType_PREBUILD
				req.Spec.Envvars = append(req.Spec.Envvars, &wsmanapi.EnvironmentVariable{
					Name:  "GITPOD_TASKS",
					Value: `[{ "init": "echo \"some output\" > someFile; sleep 20; exit 1;" }]`,
				})
				return nil
			}))
			if err != nil {
				t.Fatalf("cannot start workspace: %q", err)
			}

			t.Cleanup(func() {
				sctx, scancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer scancel()

				sapi := integration.NewComponentAPI(sctx, cfg.Namespace(), kubeconfig, cfg.Client())
				defer sapi.Done(t)

				_, err = stopWs(true, sapi)
				if err != nil {
					t.Fatal(err)
				}
			})

			_, err = integration.WaitForWorkspace(ctx, api, ws.Req.Id, func(status *wsmanapi.WorkspaceStatus) bool {
				if status.Phase != wsmanapi.WorkspacePhase_STOPPED {
					return false
				}
				if status.Conditions.HeadlessTaskFailed == "" {
					t.Logf("Conditions: %v", status.Conditions)
					t.Fatal("expected HeadlessTaskFailed condition")
				}
				return true
			})
			if err != nil {
				t.Fatalf("cannot start workspace: %q", err)
			}

			return ctx
		}).
		Feature()

	testEnv.Test(t, f)
}
