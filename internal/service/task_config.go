package service

// TaskCategory matches welfare UI blocks.
type TaskCategory string

const (
	TaskCategoryLimited     TaskCategory = "limited"
	TaskCategoryPlay        TaskCategory = "play"
	TaskCategoryDaily       TaskCategory = "daily"
	TaskCategoryAd          TaskCategory = "ad"
	TaskCategoryNewbie      TaskCategory = "newbie"
	TaskCategorySocial      TaskCategory = "social"
	TaskCategoryAchievement TaskCategory = "achievement"
)

// TaskMetric progress source.
type TaskMetric string

const (
	TaskMetricSignin7dChain       TaskMetric = "signin_7d_chain"
	TaskMetricStreakActiveDays    TaskMetric = "streak_active_days"
	TaskMetricStreakSigninDays    TaskMetric = "streak_signin_days"
	TaskMetricActiveSecDay        TaskMetric = "active_sec_day"
	TaskMetricActiveSecWeek       TaskMetric = "active_sec_week"
	TaskMetricActiveSecLifetime   TaskMetric = "active_sec_lifetime"
	TaskMetricPlayDistinct30Day   TaskMetric = "play_distinct_games_day"
	TaskMetricPlayGameSecDay      TaskMetric = "play_game_sec_day"
	TaskMetricPlayOpensLifetime   TaskMetric = "play_opens_lifetime"
	TaskMetricInviteValidDay      TaskMetric = "invite_valid_day"
	TaskMetricInviteTotal         TaskMetric = "invite_total"
	TaskMetricHasInviter          TaskMetric = "has_inviter"
	TaskMetricAdsCompleteDay      TaskMetric = "ads_complete_day"
	TaskMetricAdsCompleteWeek     TaskMetric = "ads_complete_week"
	TaskMetricAdsCompleteLifetime TaskMetric = "ads_complete_lifetime"
	TaskMetricAdFirstToday        TaskMetric = "ad_first_today"
	TaskMetricGoldEarnedDay       TaskMetric = "gold_earned_day"
	TaskMetricGoldEarnedLifetime  TaskMetric = "gold_earned_lifetime"
	TaskMetricPageViewDay         TaskMetric = "page_view_day"
	TaskMetricShareSuccessDay     TaskMetric = "share_success_day"
	TaskMetricProfileComplete     TaskMetric = "profile_complete"
	TaskMetricGuestUpgraded       TaskMetric = "guest_upgraded"
	TaskMetricAccountAgeDays      TaskMetric = "account_age_days"
	TaskMetricAccountIdleDays     TaskMetric = "account_idle_days"
	TaskMetricSignin7dRounds      TaskMetric = "signin_7d_rounds"
	TaskMetricNone                TaskMetric = "none"
)

// TaskResetPolicy period key semantics.
type TaskResetPolicy string

const (
	TaskResetDaily   TaskResetPolicy = "daily"
	TaskResetWeekly  TaskResetPolicy = "weekly"
	TaskResetOnce    TaskResetPolicy = "once"
	TaskResetRound7d TaskResetPolicy = "round_7d"
)

// TaskStatus client-facing state.
type TaskStatus string

const (
	TaskStatusLocked     TaskStatus = "locked"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusClaimable  TaskStatus = "claimable"
	TaskStatusClaimed    TaskStatus = "claimed"
)

// TaskDef welfare task definition.
type TaskDef struct {
	TaskID            string
	Tier              TaskTier
	Category          TaskCategory
	TitleKey          string
	Metric            TaskMetric
	Target            float64
	RewardGold        float64
	AdBonusGold       float64
	AdBonusPercent    float64
	ResetPolicy       TaskResetPolicy
	BizType           string
	Enabled           bool
	SortOrder         int
	PageKey           string // page_view_day
	WeekendOnly       bool
	MinIdleDays       int
	MaxAccountAgeDays int // newbie visibility: account age must be <=
	ExactAccountAge   int // account_age_days metric must match
}

// Signin7dGoldRewards day 1..7 (策划 §3.2 / §5.17).
var Signin7dGoldRewards = []float64{100, 150, 200, 260, 330, 410, 520}

// DefaultP0Tasks kept for tests referencing P0 count.
func DefaultP0Tasks() []TaskDef {
	var out []TaskDef
	for _, d := range BuildFullTaskCatalog() {
		if d.Tier == TaskTierP0 {
			out = append(out, d)
		}
	}
	return out
}
