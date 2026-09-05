package model

import "time"

type Tenant struct {
	ID             int64     `json:"id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	Status         string    `json:"status"`
	OwnerAccountID int64     `json:"owner_account_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TenantMember struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	AccountID int64     `json:"account_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Plan struct {
	ID                int64     `json:"id"`
	Code              string    `json:"code"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	PriceMonthlyCents int64     `json:"price_monthly_cents"`
	Currency          string    `json:"currency"`
	MaxMembers        int64     `json:"max_members"`
	MaxShortLinks     int64     `json:"max_short_links"`
	MaxLiveQRs        int64     `json:"max_live_qrs"`
	MaxTargetsPerLink int64     `json:"max_targets_per_link"`
	MonthlyVisits     int64     `json:"monthly_visits"`
	FeaturesJSON      string    `json:"features_json"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type TenantSubscription struct {
	ID                     int64      `json:"id"`
	TenantID               int64      `json:"tenant_id"`
	PlanID                 int64      `json:"plan_id"`
	PlanCode               string     `json:"plan_code"`
	PlanName               string     `json:"plan_name"`
	Status                 string     `json:"status"`
	Provider               string     `json:"provider"`
	ExternalCustomerID     string     `json:"external_customer_id"`
	ExternalSubscriptionID string     `json:"external_subscription_id"`
	CurrentPeriodStart     *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd       *time.Time `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd      bool       `json:"cancel_at_period_end"`
	TrialEndsAt            *time.Time `json:"trial_ends_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type SubscriptionChangeRequest struct {
	ID          int64      `json:"id"`
	TenantID    int64      `json:"tenant_id"`
	TenantName  string     `json:"tenant_name"`
	FromPlanID  int64      `json:"from_plan_id"`
	FromPlan    string     `json:"from_plan"`
	ToPlanID    int64      `json:"to_plan_id"`
	ToPlan      string     `json:"to_plan"`
	Status      string     `json:"status"`
	Note        string     `json:"note"`
	ReviewNote  string     `json:"review_note"`
	RequestedBy int64      `json:"requested_by"`
	ReviewedBy  int64      `json:"reviewed_by"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type TenantUsage struct {
	TenantID          int64  `json:"tenant_id"`
	PeriodKey         string `json:"period_key"`
	Visits            int64  `json:"visits"`
	ShortLinksCreated int64  `json:"short_links_created"`
	LiveQRsCreated    int64  `json:"live_qrs_created"`
}

type TenantQuotaSnapshot struct {
	Plan              Plan               `json:"plan"`
	Subscription      TenantSubscription `json:"subscription"`
	MembersUsed       int64              `json:"members_used"`
	ShortLinksUsed    int64              `json:"short_links_used"`
	LiveQRsUsed       int64              `json:"live_qrs_used"`
	MonthlyVisitsUsed int64              `json:"monthly_visits_used"`
	PeriodKey         string             `json:"period_key"`
}

type ApprovalEvent struct {
	ID             int64     `json:"id"`
	TenantID       int64     `json:"tenant_id"`
	ResourceType   string    `json:"resource_type"`
	ResourceID     int64     `json:"resource_id"`
	ContentVersion int64     `json:"content_version"`
	Stage          string    `json:"stage"`
	Action         string    `json:"action"`
	ActorAccountID int64     `json:"actor_account_id"`
	Note           string    `json:"note"`
	CreatedAt      time.Time `json:"created_at"`
}

type ApprovalQueueItem struct {
	TenantID       int64     `json:"tenant_id"`
	TenantName     string    `json:"tenant_name"`
	ResourceType   string    `json:"resource_type"`
	ResourceID     int64     `json:"resource_id"`
	Code           string    `json:"code"`
	Title          string    `json:"title"`
	ApprovalStatus string    `json:"approval_status"`
	ContentVersion int64     `json:"content_version"`
	OwnerAccountID int64     `json:"owner_account_id"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ShortLinkTarget struct {
	ID           int64      `json:"id"`
	TenantID     int64      `json:"tenant_id"`
	ShortLinkID  int64      `json:"short_link_id"`
	Name         string     `json:"name"`
	TargetURL    string     `json:"target_url"`
	Status       string     `json:"status"`
	Weight       int        `json:"weight"`
	SortOrder    int        `json:"sort_order"`
	StartsAt     *time.Time `json:"starts_at,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	MaxHits      int64      `json:"max_hits"`
	HitCount     int64      `json:"hit_count"`
	HealthStatus string     `json:"health_status"`
	LastHitAt    *time.Time `json:"last_hit_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ShortLinkWorkspace struct {
	ShortLink
	TenantID            int64             `json:"tenant_id"`
	RoutingStrategy     string            `json:"routing_strategy"`
	CurrentTargetCursor int64             `json:"current_target_cursor"`
	ContentVersion      int64             `json:"content_version"`
	ApprovedVersion     int64             `json:"approved_version"`
	Targets             []ShortLinkTarget `json:"targets,omitempty"`
}

type LiveQRWorkspace struct {
	LiveQR
	TenantID        int64 `json:"tenant_id"`
	ContentVersion  int64 `json:"content_version"`
	ApprovedVersion int64 `json:"approved_version"`
}

type ShortRouteDecision struct {
	TenantID  int64  `json:"tenant_id"`
	TargetID  *int64 `json:"target_id,omitempty"`
	TargetURL string `json:"target_url"`
	Strategy  string `json:"strategy"`
	Counted   bool   `json:"counted"`
}

type TenantAccess struct {
	Tenant Tenant `json:"tenant"`
	Role   string `json:"role"`
}
