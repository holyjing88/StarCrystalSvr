package service

// BuildFullTaskCatalog returns策划池 P0+P1+P2（P2 默认 tier 关）。
func BuildFullTaskCatalog() []TaskDef {
	var out []TaskDef
	add := func(d TaskDef) {
		if d.Tier == "" {
			d.Tier = TaskTierP1
		}
		if d.BizType == "" {
			d.BizType = "task"
		}
		if d.ExactAccountAge == 0 && d.TaskID != "newbie_day1_welcome" {
			d.ExactAccountAge = -1
		}
		out = append(out, d)
	}

	// --- P0 §5.17 ---
	add(TaskDef{TaskID: "signin_7d", Tier: TaskTierP0, Category: TaskCategoryLimited, TitleKey: "welfare.task.signin_7d", Metric: TaskMetricSignin7dChain, Target: 1, AdBonusPercent: 0.5, ResetPolicy: TaskResetRound7d, BizType: "signin", Enabled: true, SortOrder: 1})
	add(TaskDef{TaskID: "streak_play_3d", Tier: TaskTierP0, Category: TaskCategoryLimited, TitleKey: "welfare.task.streak_play_3d", Metric: TaskMetricStreakActiveDays, Target: 3, RewardGold: 150, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 2})
	add(TaskDef{TaskID: "daily_free_claim", Tier: TaskTierP0, Category: TaskCategoryDaily, TitleKey: "welfare.task.daily_free", Metric: TaskMetricNone, Target: 1, RewardGold: 50, AdBonusGold: 30, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 1})
	add(TaskDef{TaskID: "daily_play_sessions", Tier: TaskTierP0, Category: TaskCategoryDaily, TitleKey: "welfare.task.daily_play_sessions", Metric: TaskMetricPlayDistinct30Day, Target: 3, RewardGold: 100, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 3})
	add(TaskDef{TaskID: "daily_invite", Tier: TaskTierP0, Category: TaskCategoryDaily, TitleKey: "welfare.task.daily_invite", Metric: TaskMetricInviteValidDay, Target: 1, RewardGold: 200, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 4})
	add(TaskDef{TaskID: "play_daily_60s", Tier: TaskTierP0, Category: TaskCategoryPlay, TitleKey: "welfare.task.play_60s", Metric: TaskMetricActiveSecDay, Target: 60, RewardGold: 80, AdBonusGold: 40, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 1})
	add(TaskDef{TaskID: "play_milestone_300s", Tier: TaskTierP0, Category: TaskCategoryPlay, TitleKey: "welfare.task.play_300s", Metric: TaskMetricActiveSecDay, Target: 300, RewardGold: 100, AdBonusGold: 50, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 2})
	add(TaskDef{TaskID: "ad_daily_1", Tier: TaskTierP0, Category: TaskCategoryAd, TitleKey: "welfare.task.ad_daily_1", Metric: TaskMetricAdsCompleteDay, Target: 1, RewardGold: 50, ResetPolicy: TaskResetDaily, BizType: "ad", Enabled: true, SortOrder: 1})
	add(TaskDef{TaskID: "ad_daily_3", Tier: TaskTierP0, Category: TaskCategoryAd, TitleKey: "welfare.task.ad_daily_3", Metric: TaskMetricAdsCompleteDay, Target: 3, RewardGold: 150, ResetPolicy: TaskResetDaily, BizType: "ad", Enabled: true, SortOrder: 2})
	add(TaskDef{TaskID: "ad_first_today", Tier: TaskTierP0, Category: TaskCategoryAd, TitleKey: "welfare.task.ad_first", Metric: TaskMetricAdFirstToday, Target: 1, RewardGold: 80, ResetPolicy: TaskResetDaily, BizType: "ad", Enabled: true, SortOrder: 3})

	// --- P1 limited ---
	add(TaskDef{TaskID: "streak_play_7d", Tier: TaskTierP1, Category: TaskCategoryLimited, TitleKey: "welfare.task.streak_play_7d", Metric: TaskMetricStreakActiveDays, Target: 7, RewardGold: 400, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 3})
	add(TaskDef{TaskID: "streak_signin_7d", Tier: TaskTierP1, Category: TaskCategoryLimited, TitleKey: "welfare.task.streak_signin_7d", Metric: TaskMetricStreakSigninDays, Target: 7, RewardGold: 300, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 4})
	add(TaskDef{TaskID: "comeback_gift", Tier: TaskTierP1, Category: TaskCategoryLimited, TitleKey: "welfare.task.comeback", Metric: TaskMetricAccountIdleDays, Target: 1, RewardGold: 300, ResetPolicy: TaskResetOnce, MinIdleDays: 3, Enabled: true, SortOrder: 5})
	add(TaskDef{TaskID: "weekend_limited", Tier: TaskTierP1, Category: TaskCategoryLimited, TitleKey: "welfare.task.weekend", Metric: TaskMetricNone, Target: 1, RewardGold: 100, ResetPolicy: TaskResetDaily, WeekendOnly: true, Enabled: true, SortOrder: 6})

	// --- P1 play ---
	add(TaskDef{TaskID: "play_milestone_900s", Tier: TaskTierP1, Category: TaskCategoryPlay, TitleKey: "welfare.task.play_900s", Metric: TaskMetricActiveSecDay, Target: 900, RewardGold: 200, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 3})
	add(TaskDef{TaskID: "play_milestone_1800s", Tier: TaskTierP1, Category: TaskCategoryPlay, TitleKey: "welfare.task.play_1800s", Metric: TaskMetricActiveSecDay, Target: 1800, RewardGold: 360, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 4})
	add(TaskDef{TaskID: "play_weekly_600s", Tier: TaskTierP1, Category: TaskCategoryPlay, TitleKey: "welfare.task.play_weekly_600s", Metric: TaskMetricActiveSecWeek, Target: 600, RewardGold: 150, ResetPolicy: TaskResetWeekly, Enabled: true, SortOrder: 5})
	add(TaskDef{TaskID: "play_weekly_3600s", Tier: TaskTierP1, Category: TaskCategoryPlay, TitleKey: "welfare.task.play_weekly_3600s", Metric: TaskMetricActiveSecWeek, Target: 3600, RewardGold: 500, ResetPolicy: TaskResetWeekly, Enabled: true, SortOrder: 6})
	add(TaskDef{TaskID: "play_distinct_3", Tier: TaskTierP1, Category: TaskCategoryPlay, TitleKey: "welfare.task.play_distinct_3", Metric: TaskMetricPlayDistinct30Day, Target: 3, RewardGold: 120, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 7})
	add(TaskDef{TaskID: "play_distinct_5", Tier: TaskTierP1, Category: TaskCategoryPlay, TitleKey: "welfare.task.play_distinct_5", Metric: TaskMetricPlayDistinct30Day, Target: 5, RewardGold: 200, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 8})
	add(TaskDef{TaskID: "play_featured_game", Tier: TaskTierP1, Category: TaskCategoryPlay, TitleKey: "welfare.task.play_featured", Metric: TaskMetricPlayGameSecDay, Target: 180, RewardGold: 150, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 9})

	// --- P1 daily ---
	add(TaskDef{TaskID: "daily_watch_ad_1", Tier: TaskTierP1, Category: TaskCategoryDaily, TitleKey: "welfare.task.daily_watch_ad_1", Metric: TaskMetricAdsCompleteDay, Target: 1, RewardGold: 60, ResetPolicy: TaskResetDaily, BizType: "ad", Enabled: true, SortOrder: 5})
	add(TaskDef{TaskID: "daily_watch_ad_3", Tier: TaskTierP1, Category: TaskCategoryDaily, TitleKey: "welfare.task.daily_watch_ad_3", Metric: TaskMetricAdsCompleteDay, Target: 3, RewardGold: 180, ResetPolicy: TaskResetDaily, BizType: "ad", Enabled: true, SortOrder: 6})
	add(TaskDef{TaskID: "daily_earn_gold_500", Tier: TaskTierP1, Category: TaskCategoryDaily, TitleKey: "welfare.task.daily_earn_gold", Metric: TaskMetricGoldEarnedDay, Target: 500, RewardGold: 100, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 7})
	add(TaskDef{TaskID: "daily_visit_rank", Tier: TaskTierP1, Category: TaskCategoryDaily, TitleKey: "welfare.task.daily_visit_rank", Metric: TaskMetricPageViewDay, Target: 1, RewardGold: 30, ResetPolicy: TaskResetDaily, PageKey: "rank", Enabled: true, SortOrder: 8})
	add(TaskDef{TaskID: "daily_visit_welfare", Tier: TaskTierP1, Category: TaskCategoryDaily, TitleKey: "welfare.task.daily_visit_welfare", Metric: TaskMetricPageViewDay, Target: 1, RewardGold: 20, ResetPolicy: TaskResetDaily, PageKey: "welfare", Enabled: true, SortOrder: 9})
	add(TaskDef{TaskID: "daily_share", Tier: TaskTierP1, Category: TaskCategoryDaily, TitleKey: "welfare.task.daily_share", Metric: TaskMetricShareSuccessDay, Target: 1, RewardGold: 80, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 10})
	add(TaskDef{TaskID: "daily_set_nickname", Tier: TaskTierP1, Category: TaskCategoryDaily, TitleKey: "welfare.task.daily_set_nickname", Metric: TaskMetricProfileComplete, Target: 1, RewardGold: 50, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 11})
	add(TaskDef{TaskID: "daily_bind_phone", Tier: TaskTierP1, Category: TaskCategoryDaily, TitleKey: "welfare.task.daily_bind_phone", Metric: TaskMetricGuestUpgraded, Target: 1, RewardGold: 300, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 12})

	// --- P1 ad ---
	add(TaskDef{TaskID: "ad_daily_5", Tier: TaskTierP1, Category: TaskCategoryAd, TitleKey: "welfare.task.ad_daily_5", Metric: TaskMetricAdsCompleteDay, Target: 5, RewardGold: 280, ResetPolicy: TaskResetDaily, BizType: "ad", Enabled: true, SortOrder: 4})
	add(TaskDef{TaskID: "ad_weekly_15", Tier: TaskTierP1, Category: TaskCategoryAd, TitleKey: "welfare.task.ad_weekly_15", Metric: TaskMetricAdsCompleteWeek, Target: 15, RewardGold: 400, ResetPolicy: TaskResetWeekly, BizType: "ad", Enabled: true, SortOrder: 5})

	// --- P1 newbie ---
	add(TaskDef{TaskID: "newbie_day1_welcome", Tier: TaskTierP1, Category: TaskCategoryNewbie, TitleKey: "welfare.task.newbie_day1", Metric: TaskMetricAccountAgeDays, Target: 1, RewardGold: 200, ResetPolicy: TaskResetOnce, MaxAccountAgeDays: 0, ExactAccountAge: 0, Enabled: true, SortOrder: 1})
	add(TaskDef{TaskID: "newbie_first_play", Tier: TaskTierP1, Category: TaskCategoryNewbie, TitleKey: "welfare.task.newbie_first_play", Metric: TaskMetricPlayOpensLifetime, Target: 1, RewardGold: 100, ResetPolicy: TaskResetOnce, MaxAccountAgeDays: 7, Enabled: true, SortOrder: 2})
	add(TaskDef{TaskID: "newbie_first_60s", Tier: TaskTierP1, Category: TaskCategoryNewbie, TitleKey: "welfare.task.newbie_first_60s", Metric: TaskMetricActiveSecLifetime, Target: 60, RewardGold: 150, ResetPolicy: TaskResetOnce, MaxAccountAgeDays: 7, Enabled: true, SortOrder: 3})
	add(TaskDef{TaskID: "newbie_first_ad", Tier: TaskTierP1, Category: TaskCategoryNewbie, TitleKey: "welfare.task.newbie_first_ad", Metric: TaskMetricAdsCompleteLifetime, Target: 1, RewardGold: 120, ResetPolicy: TaskResetOnce, MaxAccountAgeDays: 7, Enabled: true, SortOrder: 4})
	add(TaskDef{TaskID: "newbie_day3_check", Tier: TaskTierP1, Category: TaskCategoryNewbie, TitleKey: "welfare.task.newbie_day3", Metric: TaskMetricAccountAgeDays, Target: 3, RewardGold: 150, ResetPolicy: TaskResetOnce, MaxAccountAgeDays: 7, Enabled: true, SortOrder: 5})
	add(TaskDef{TaskID: "newbie_day7_graduate", Tier: TaskTierP1, Category: TaskCategoryNewbie, TitleKey: "welfare.task.newbie_day7", Metric: TaskMetricAccountAgeDays, Target: 7, RewardGold: 500, ResetPolicy: TaskResetOnce, MaxAccountAgeDays: 14, Enabled: true, SortOrder: 6})

	// --- P1 social ---
	add(TaskDef{TaskID: "social_invite_3_total", Tier: TaskTierP1, Category: TaskCategorySocial, TitleKey: "welfare.task.social_invite_3", Metric: TaskMetricInviteTotal, Target: 3, RewardGold: 500, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 1})
	add(TaskDef{TaskID: "social_invite_10_total", Tier: TaskTierP1, Category: TaskCategorySocial, TitleKey: "welfare.task.social_invite_10", Metric: TaskMetricInviteTotal, Target: 10, RewardGold: 2000, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 2})
	add(TaskDef{TaskID: "social_invitee_reward", Tier: TaskTierP1, Category: TaskCategorySocial, TitleKey: "welfare.task.social_invitee", Metric: TaskMetricHasInviter, Target: 1, RewardGold: 100, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 3})
	add(TaskDef{TaskID: "social_share_moment", Tier: TaskTierP1, Category: TaskCategorySocial, TitleKey: "welfare.task.social_share", Metric: TaskMetricShareSuccessDay, Target: 1, RewardGold: 80, ResetPolicy: TaskResetDaily, Enabled: true, SortOrder: 4})

	// --- P2 achievement (catalog only) ---
	add(TaskDef{TaskID: "ach_gold_10k", Tier: TaskTierP2, Category: TaskCategoryAchievement, TitleKey: "welfare.task.ach_gold_10k", Metric: TaskMetricGoldEarnedLifetime, Target: 10000, RewardGold: 200, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 1})
	add(TaskDef{TaskID: "ach_gold_100k", Tier: TaskTierP2, Category: TaskCategoryAchievement, TitleKey: "welfare.task.ach_gold_100k", Metric: TaskMetricGoldEarnedLifetime, Target: 100000, RewardGold: 1000, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 2})
	add(TaskDef{TaskID: "ach_play_100h", Tier: TaskTierP2, Category: TaskCategoryAchievement, TitleKey: "welfare.task.ach_play_100h", Metric: TaskMetricActiveSecLifetime, Target: 360000, RewardGold: 800, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 3})
	add(TaskDef{TaskID: "ach_ad_100", Tier: TaskTierP2, Category: TaskCategoryAchievement, TitleKey: "welfare.task.ach_ad_100", Metric: TaskMetricAdsCompleteLifetime, Target: 100, RewardGold: 300, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 4})
	add(TaskDef{TaskID: "ach_signin_rounds_5", Tier: TaskTierP2, Category: TaskCategoryAchievement, TitleKey: "welfare.task.ach_signin_rounds", Metric: TaskMetricSignin7dRounds, Target: 5, RewardGold: 500, ResetPolicy: TaskResetOnce, Enabled: true, SortOrder: 5})

	return out
}
