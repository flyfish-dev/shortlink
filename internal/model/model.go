package model

import "time"

type ShortLink struct {
	ID             int64      `json:"id"`
	Code           string     `json:"code"`
	Title          string     `json:"title"`
	TargetURL      string     `json:"target_url"`
	Status         string     `json:"status"`
	ApprovalStatus string     `json:"approval_status"`
	ApprovedAt     *time.Time `json:"approved_at,omitempty"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	ReviewNote     string     `json:"review_note"`
	RedirectType   int        `json:"redirect_type"`
	StartsAt       *time.Time `json:"starts_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	MaxVisits      int64      `json:"max_visits"`
	VisitCount     int64      `json:"visit_count"`
	FallbackURL    string     `json:"fallback_url"`
	Remark         string     `json:"remark"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type LiveQR struct {
	ID               int64        `json:"id"`
	Code             string       `json:"code"`
	Title            string       `json:"title"`
	Description      string       `json:"description"`
	Status           string       `json:"status"`
	ApprovalStatus   string       `json:"approval_status"`
	ApprovedAt       *time.Time   `json:"approved_at,omitempty"`
	ReviewedAt       *time.Time   `json:"reviewed_at,omitempty"`
	ReviewNote       string       `json:"review_note"`
	RotationStrategy string       `json:"rotation_strategy"`
	CurrentCursor    int64        `json:"current_cursor"`
	VisitCount       int64        `json:"visit_count"`
	GuideTitle       string       `json:"guide_title"`
	GuideText        string       `json:"guide_text"`
	FallbackURL      string       `json:"fallback_url"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	Items            []LiveQRItem `json:"items,omitempty"`
}

type LiveQRItem struct {
	ID             int64      `json:"id"`
	LiveQRID       int64      `json:"live_qr_id"`
	Title          string     `json:"title"`
	QRImageURL     string     `json:"qr_image_url"`
	TargetURL      string     `json:"target_url"`
	Status         string     `json:"status"`
	ApprovalStatus string     `json:"approval_status"`
	ApprovedAt     *time.Time `json:"approved_at,omitempty"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	ReviewNote     string     `json:"review_note"`
	StartsAt       *time.Time `json:"starts_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	MaxViews       int64      `json:"max_views"`
	ViewCount      int64      `json:"view_count"`
	SortOrder      int        `json:"sort_order"`
	Weight         int        `json:"weight"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type VisitLog struct {
	ID           int64     `json:"id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   int64     `json:"resource_id"`
	ItemID       *int64    `json:"item_id,omitempty"`
	Code         string    `json:"code"`
	EventType    string    `json:"event_type"`
	Status       string    `json:"status"`
	TargetURL    string    `json:"target_url"`
	IP           string    `json:"ip"`
	IPHash       string    `json:"-"`
	UserAgent    string    `json:"user_agent"`
	Referer      string    `json:"referer"`
	DeviceType   string    `json:"device_type"`
	Browser      string    `json:"browser"`
	OS           string    `json:"os"`
	CreatedAt    time.Time `json:"created_at"`
}

type AdminAccount struct {
	ID                int64     `json:"id"`
	Email             string    `json:"email"`
	Name              string    `json:"name"`
	RecoveryKeyHash   string    `json:"-"`
	RecoveryKeyCipher string    `json:"-"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type AdminDevice struct {
	ID            int64      `json:"id"`
	AccountID     int64      `json:"account_id"`
	Label         string     `json:"label"`
	BrowserHash   string     `json:"-"`
	IPHash        string     `json:"-"`
	IPLast        string     `json:"ip_last"`
	UserAgentLast string     `json:"user_agent_last"`
	CreatedAt     time.Time  `json:"created_at"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}

type SystemSettings struct {
	Installed       bool   `json:"installed"`
	AppName         string `json:"app_name"`
	AppNameZH       string `json:"app_name_zh"`
	AppNameEN       string `json:"app_name_en"`
	BaseURL         string `json:"base_url"`
	DefaultLocale   string `json:"default_locale"`
	LoginMode       string `json:"login_mode"`
	AdminEmail      string `json:"admin_email"`
	SMTPEnabled     bool   `json:"smtp_enabled"`
	SMTPHost        string `json:"smtp_host"`
	SMTPPort        int    `json:"smtp_port"`
	SMTPSecurity    string `json:"smtp_security"`
	SMTPUsername    string `json:"smtp_username"`
	SMTPFrom        string `json:"smtp_from"`
	SMTPPasswordSet bool   `json:"smtp_password_set"`
}

type MagicLoginToken struct {
	ID        int64      `json:"id"`
	AccountID int64      `json:"account_id"`
	Email     string     `json:"email"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	IP        string     `json:"ip"`
	CreatedAt time.Time  `json:"created_at"`
}

type Overview struct {
	ShortLinks        int64 `json:"short_links"`
	ShortPending      int64 `json:"short_pending"`
	LiveQRs           int64 `json:"live_qrs"`
	LivePending       int64 `json:"live_pending"`
	LiveItemsPending  int64 `json:"live_items_pending"`
	VisitsToday       int64 `json:"visits_today"`
	VisitsTotal       int64 `json:"visits_total"`
	LiveItemsActive   int64 `json:"live_items_active"`
	SMTPConfigured    bool  `json:"smtp_configured"`
	BaseURLConfigured bool  `json:"base_url_configured"`
}

type DateStat struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type DimStat struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type StatsBundle struct {
	Total     int64      `json:"total"`
	UniqueIPs int64      `json:"unique_ips"`
	ByDate    []DateStat `json:"by_date"`
	ByDevice  []DimStat  `json:"by_device"`
	ByBrowser []DimStat  `json:"by_browser"`
	Recent    []VisitLog `json:"recent"`
}
