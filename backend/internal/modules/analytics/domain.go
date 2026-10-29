// Package analytics is a read-only reporting module. Unlike other modules it reads
// several modules' tables directly (dashboards need aggregate SQL, not N service calls);
// every query is still tenant-filtered and runs inside db.WithTenantTx.
package analytics

import (
	"time"

	"github.com/google/uuid"
)

type DashboardStats struct {
	ActiveSubscriptions           int
	ExpiringIn7Days               int
	ExpiredUnrenewed30Days        int
	TotalCustomers                int
	NewCustomersThisMonth         int
	MRR                           int64
	RevenueThisMonth              int64
	CostThisMonth                 int64
	ProfitThisMonth               int64
	TotalDue                      int64
	PendingPayments               int
	OpenTickets                   int
	OpenConversations             int
	FreeSlots                     int
	MasterAccountsExpiringIn7Days int
	Currency                      string
}

type SeriesPoint struct {
	Date  time.Time
	Value int64
	Count int
}

type ProductStat struct {
	ProductID           uuid.UUID
	ActiveSubscriptions int
	Revenue             int64
	Cost                int64
	Profit              int64
	RenewalRate         float64
}

type ChannelStat struct {
	Platform      string
	Conversations int
	Customers     int
	Orders        int
}

// At-risk reasons, in priority order.
const (
	ReasonExpiringSoon   = "EXPIRING_SOON"
	ReasonOverdueInvoice = "OVERDUE_INVOICE"
	ReasonHasDue         = "HAS_DUE"
)

type AtRiskCustomer struct {
	CustomerID uuid.UUID
	Reason     string
	ExpiresAt  *time.Time
	DueBalance int64
}

// RenewalRate = renewed / ended, 0 when nothing ended.
func RenewalRate(renewed, ended int) float64 {
	if ended <= 0 {
		return 0
	}
	r := float64(renewed) / float64(ended)
	if r > 1 {
		return 1
	}
	return float64(int(r*1000+0.5)) / 1000
}

// ClampDays bounds a reporting window.
func ClampDays(d int) int {
	switch {
	case d <= 0:
		return 30
	case d > 366:
		return 366
	}
	return d
}
