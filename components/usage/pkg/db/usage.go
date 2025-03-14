// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UsageKind string

const (
	WorkspaceInstanceUsageKind UsageKind = "workspaceinstance"
	InvoiceUsageKind           UsageKind = "invoice"
)

func NewCreditCents(n float64) CreditCents {
	inCents := n * 100
	return CreditCents(int64(math.Round(inCents)))
}

type CreditCents int64

func (cc CreditCents) ToCredits() float64 {
	return float64(cc) / 100
}

type Usage struct {
	ID                  uuid.UUID      `gorm:"primary_key;column:id;type:char;size:36;" json:"id"`
	AttributionID       AttributionID  `gorm:"column:attributionId;type:varchar;size:255;" json:"attributionId"`
	Description         string         `gorm:"column:description;type:varchar;size:255;" json:"description"`
	CreditCents         CreditCents    `gorm:"column:creditCents;type:bigint;" json:"creditCents"`
	EffectiveTime       VarcharTime    `gorm:"column:effectiveTime;type:varchar;size:255;" json:"effectiveTime"`
	Kind                UsageKind      `gorm:"column:kind;type:char;size:10;" json:"kind"`
	WorkspaceInstanceID uuid.UUID      `gorm:"column:workspaceInstanceId;type:char;size:36;" json:"workspaceInstanceId"`
	Draft               bool           `gorm:"column:draft;type:boolean;" json:"draft"`
	Metadata            datatypes.JSON `gorm:"column:metadata;type:text;size:65535" json:"metadata"`
}

func (u *Usage) SetMetadataWithWorkspaceInstance(data WorkspaceInstanceUsageData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize workspace instance usage data into json: %w", err)
	}

	u.Metadata = b
	return nil
}

func (u *Usage) GetMetadataAsWorkspaceInstanceData() (WorkspaceInstanceUsageData, error) {
	var data WorkspaceInstanceUsageData
	err := json.Unmarshal(u.Metadata, &data)
	if err != nil {
		return WorkspaceInstanceUsageData{}, fmt.Errorf("failed unmarshal metadata into wokrspace instance data: %w", err)
	}

	return data, nil
}

// WorkspaceInstanceUsageData represents the shape of metadata for usage entries of kind "workspaceinstance"
// the equivalent TypeScript definition is maintained in `components/gitpod-protocol/src/usage.ts“
type WorkspaceInstanceUsageData struct {
	WorkspaceId    string        `json:"workspaceId"`
	WorkspaceType  WorkspaceType `json:"workspaceType"`
	WorkspaceClass string        `json:"workspaceClass"`
	ContextURL     string        `json:"contextURL"`
	StartTime      string        `json:"startTime"`
	EndTime        string        `json:"endTime"`
	UserID         uuid.UUID     `json:"userId"`
	UserName       string        `json:"userName"`
	UserAvatarURL  string        `json:"userAvatarURL"`
}

type FindUsageResult struct {
	UsageEntries []Usage
}

// TableName sets the insert table name for this struct type
func (u *Usage) TableName() string {
	return "d_b_usage"
}

func InsertUsage(ctx context.Context, conn *gorm.DB, records ...Usage) error {
	return conn.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(records, 1000).Error
}

func UpdateUsage(ctx context.Context, conn *gorm.DB, records ...Usage) error {
	for _, record := range records {
		err := conn.WithContext(ctx).Save(record).Error
		if err != nil {
			return fmt.Errorf("failed to update usage record ID: %s: %w", record.ID, err)
		}
	}

	return nil
}

func FindAllDraftUsage(ctx context.Context, conn *gorm.DB) ([]Usage, error) {
	var usageRecords []Usage
	var usageRecordsBatch []Usage

	result := conn.WithContext(ctx).
		Where("draft = TRUE").
		Order("effectiveTime DESC").
		FindInBatches(&usageRecordsBatch, 1000, func(_ *gorm.DB, _ int) error {
			usageRecords = append(usageRecords, usageRecordsBatch...)
			return nil
		})
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get usage records: %s", result.Error)
	}
	return usageRecords, nil
}

type FindUsageParams struct {
	AttributionId AttributionID
	From, To      time.Time
	ExcludeDrafts bool
	Order         Order
	Offset, Limit int64
}

func FindUsage(ctx context.Context, conn *gorm.DB, params *FindUsageParams) ([]Usage, error) {
	var usageRecords []Usage
	var usageRecordsBatch []Usage

	db := conn.WithContext(ctx).
		Where("attributionId = ?", params.AttributionId).
		Where("effectiveTime >= ? AND effectiveTime < ?", TimeToISO8601(params.From), TimeToISO8601(params.To))
	if params.ExcludeDrafts {
		db = db.Where("draft = ?", false)
	}
	db = db.Order(fmt.Sprintf("effectiveTime %s", params.Order.ToSQL()))
	if params.Offset != 0 {
		db = db.Offset(int(params.Offset))
	}
	if params.Limit != 0 {
		db = db.Limit(int(params.Limit))
	}

	result := db.FindInBatches(&usageRecordsBatch, 1000, func(_ *gorm.DB, _ int) error {
		usageRecords = append(usageRecords, usageRecordsBatch...)
		return nil
	})
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get usage records: %s", result.Error)
	}
	return usageRecords, nil
}

type UsageSummary struct {
	NumRecordsInRange         int
	CreditCentsBalanceAtStart int64
	CreditCentsBalanceAtEnd   int64
}

func GetUsageSummary(ctx context.Context, conn *gorm.DB, attributionId AttributionID, from, to time.Time, excludeDrafts bool) (*UsageSummary, error) {
	db := conn.WithContext(ctx)
	query1 := db.Table((&Usage{}).TableName()).
		Select("sum(creditCents) as creditCentsBalanceAtStart").
		Where("attributionId = ?", attributionId).
		Where("effectiveTime < ?", TimeToISO8601(from))
	if excludeDrafts {
		query1 = query1.Where("draft = ?", false)
	}
	var creditCentsBalanceAtStart sql.NullInt64
	err := query1.Row().Scan(&creditCentsBalanceAtStart)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage meta data: %s", err)
	}

	query2 := db.Table((&Usage{}).TableName()).
		Select("sum(creditCents) as creditCentsBalanceInPeriod", "count(id) as numRecordsInRange").
		Where("attributionId = ?", attributionId).
		Where("? <= effectiveTime AND effectiveTime < ?", TimeToISO8601(from), TimeToISO8601(to))
	if excludeDrafts {
		query2 = query2.Where("draft = ?", false)
	}
	var creditCentsBalanceInPeriod sql.NullInt64
	var numRecordsInRange sql.NullInt32
	err = query2.Row().Scan(&creditCentsBalanceInPeriod, &numRecordsInRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage meta data: %s", err)
	}
	return &UsageSummary{
		NumRecordsInRange:         int(numRecordsInRange.Int32),
		CreditCentsBalanceAtStart: creditCentsBalanceAtStart.Int64,
		CreditCentsBalanceAtEnd:   creditCentsBalanceAtStart.Int64 + creditCentsBalanceInPeriod.Int64,
	}, nil
}

type Balance struct {
	AttributionID AttributionID `gorm:"column:attributionId;type:varchar;size:255;" json:"attributionId"`
	CreditCents   CreditCents   `gorm:"column:creditCents;type:bigint;" json:"creditCents"`
}

func ListBalance(ctx context.Context, conn *gorm.DB) ([]Balance, error) {
	var balances []Balance
	rows, err := conn.WithContext(ctx).
		Model(&Usage{}).
		Select("attributionId as attributionId, sum(creditCents) as creditCents").
		Group("attributionId").
		Order("attributionId").
		Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows for list balance query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var balance Balance
		err = conn.ScanRows(rows, &balance)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row into Balance struct: %w", err)
		}
		balances = append(balances, balance)
	}

	return balances, nil
}
