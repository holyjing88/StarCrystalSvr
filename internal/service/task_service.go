package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"starcrystal/server/internal/store"
)

const taskAdProofTTL = 5 * time.Minute

// WelfareTaskItem one row in GET /tasks/welfare.
type WelfareTaskItem struct {
	TaskID      string     `json:"taskId"`
	Category    string     `json:"category"`
	TitleKey    string     `json:"titleKey"`
	DescKey     string     `json:"descKey,omitempty"`
	Metric      string     `json:"metric"`
	Target      float64    `json:"target"`
	Progress    float64    `json:"progress"`
	RewardGold  float64    `json:"rewardGold"`
	AdBonusGold float64    `json:"adBonusGold,omitempty"`
	Status      TaskStatus `json:"status"`
	SortOrder   int        `json:"sortOrder"`
}

// Signin7dDTO sign-in block for welfare GET.
type Signin7dDTO struct {
	Chain         int       `json:"chain"`
	LastClaimYmd  int       `json:"lastClaimYmd"`
	RoundStartYmd int       `json:"roundStartYmd"`
	TodayYmd      int       `json:"todayYmd"`
	CanClaim      bool      `json:"canClaim"`
	DayRewards    []float64 `json:"dayRewards"`
	NextDayIndex  int       `json:"nextDayIndex"`
}

// WelfareTasksData GET /tasks/welfare payload.
type WelfareTasksData struct {
	TodayYmd   int                          `json:"todayYmd"`
	TierPolicy TaskTierPolicy               `json:"tierPolicy"`
	Signin7d   Signin7dDTO                  `json:"signin7d"`
	Tasks      []WelfareTaskItem            `json:"tasks"`
	ByCategory map[string][]WelfareTaskItem `json:"byCategory"`
}

// TaskClaimResult POST /tasks/claim success data.
type TaskClaimResult struct {
	TaskID            string       `json:"taskId"`
	GrantedGold       float64      `json:"grantedGold"`
	CurGold           float64      `json:"curgold"`
	DailyCapRemaining float64      `json:"dailyCapRemaining"`
	Status            TaskStatus   `json:"status"`
	Signin7d          *Signin7dDTO `json:"signin7d,omitempty"`
}

// TaskService welfare task engine.
type TaskService struct {
	store  TaskProgressStore
	ledger *GoldLedgerService
	repo   store.PlayerRepository
	tz     string
}

func NewTaskService(store TaskProgressStore, ledger *GoldLedgerService, repo store.PlayerRepository) *TaskService {
	tz := defaultGoldTZ
	if ledger != nil {
		tz = ledger.tz
	}
	if store == nil {
		store = NewMemoryTaskStore()
	}
	return &TaskService{store: store, ledger: ledger, repo: repo, tz: tz}
}

func (s *TaskService) todayYmd() int {
	return ymdFromTime(GoldNow(s.tz))
}

func ymdFromTime(t time.Time) int {
	v, _ := strconv.Atoi(GoldYYYYMMDD(t))
	return v
}

func calendarDaysBetween(ymdA, ymdB int) int {
	if ymdA <= 0 || ymdB <= 0 {
		return math.MaxInt32
	}
	tA := parseYmd(ymdA)
	tB := parseYmd(ymdB)
	if tA.IsZero() || tB.IsZero() {
		return math.MaxInt32
	}
	d := tB.Sub(tA)
	return int(d.Hours() / 24)
}

func parseYmd(ymd int) time.Time {
	s := strconv.Itoa(ymd)
	if len(s) != 8 {
		return time.Time{}
	}
	loc := GoldLocation(defaultGoldTZ)
	t, err := time.ParseInLocation("20060102", s, loc)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (s *TaskService) reconcileSignin(st *Signin7dState, today int) {
	if st.Chain >= 7 && st.LastClaimYmd > 0 {
		gap := calendarDaysBetween(st.LastClaimYmd, today)
		if gap >= 1 && st.LastClaimYmd < today {
			st.Chain = 0
			return
		}
	}
	if st.LastClaimYmd > 0 {
		gap := calendarDaysBetween(st.LastClaimYmd, today)
		if gap > 1 {
			st.Chain = 0
		}
	}
}

func (s *TaskService) loadSignin(ctx context.Context, accountID string) (Signin7dState, error) {
	st, err := s.store.GetSignin7d(ctx, accountID)
	if err != nil {
		return Signin7dState{}, err
	}
	today := s.todayYmd()
	s.reconcileSignin(&st, today)
	return st, nil
}

func (s *TaskService) buildSigninDTO(ctx context.Context, accountID string) (Signin7dDTO, error) {
	st, err := s.loadSignin(ctx, accountID)
	if err != nil {
		return Signin7dDTO{}, err
	}
	today := s.todayYmd()
	can := st.LastClaimYmd != today && st.Chain < 7
	if st.LastClaimYmd > 0 && st.LastClaimYmd != today {
		gap := calendarDaysBetween(st.LastClaimYmd, today)
		if gap > 1 && st.Chain > 0 {
			// after reconcile chain may be 0
		}
	}
	next := st.Chain
	if next > 6 {
		next = 6
	}
	rewards := make([]float64, len(Signin7dGoldRewards))
	copy(rewards, Signin7dGoldRewards)
	return Signin7dDTO{
		Chain:         st.Chain,
		LastClaimYmd:  st.LastClaimYmd,
		RoundStartYmd: st.RoundStartYmd,
		TodayYmd:      today,
		CanClaim:      can,
		DayRewards:    rewards,
		NextDayIndex:  next,
	}, nil
}

func (s *TaskService) claimKey(def TaskDef, accountID string, today int, signin *Signin7dState) string {
	switch def.ResetPolicy {
	case TaskResetDaily:
		return fmt.Sprintf("daily:%d:%s:%s", today, def.TaskID, accountID)
	case TaskResetOnce:
		return fmt.Sprintf("once:%s:%s", def.TaskID, accountID)
	case TaskResetRound7d:
		round := 0
		if signin != nil {
			round = signin.RoundStartYmd
		}
		return fmt.Sprintf("signin:%d:%d:%s", round, today, accountID)
	case TaskResetWeekly:
		week := CurrentActivityWeekID(GoldNow(s.tz))
		return fmt.Sprintf("weekly:%s:%s:%s", week, def.TaskID, accountID)
	default:
		return fmt.Sprintf("daily:%d:%s:%s", today, def.TaskID, accountID)
	}
}

func (s *TaskService) metricProgress(ctx context.Context, accountID string, def TaskDef, today int, signin Signin7dState, uc TaskUserContext) (float64, error) {
	weekID := CurrentActivityWeekID(GoldNow(s.tz))
	switch def.Metric {
	case TaskMetricNone:
		if def.TaskID == "comeback_gift" {
			if uc.AccountIdleDays >= def.MinIdleDays {
				return 1, nil
			}
			return 0, nil
		}
		return 1, nil
	case TaskMetricSignin7dChain:
		return float64(signin.Chain), nil
	case TaskMetricActiveSecDay:
		v, err := s.store.GetActiveSecDay(ctx, today, accountID)
		return float64(v), err
	case TaskMetricPlayDistinct30Day:
		n, err := s.store.CountDistinctGames30Day(ctx, today, accountID)
		return float64(n), err
	case TaskMetricInviteValidDay:
		v, err := s.store.GetInviteValidDay(ctx, today, accountID)
		return float64(v), err
	case TaskMetricAdsCompleteDay:
		v, err := s.store.GetAdsCompleteDay(ctx, today, accountID)
		return float64(v), err
	case TaskMetricAdFirstToday:
		v, err := s.store.GetAdsCompleteDay(ctx, today, accountID)
		if err != nil {
			return 0, err
		}
		if v >= 1 {
			return 1, nil
		}
		return 0, nil
	case TaskMetricStreakActiveDays:
		n, err := s.store.GetStreakActiveDays(ctx, accountID)
		return float64(n), err
	case TaskMetricStreakSigninDays:
		n, err := s.store.GetStreakSigninDays(ctx, accountID)
		return float64(n), err
	case TaskMetricActiveSecWeek:
		v, err := s.store.GetActiveSecWeek(ctx, weekID, accountID)
		return float64(v), err
	case TaskMetricActiveSecLifetime:
		v, err := s.store.GetActiveSecLifetime(ctx, accountID)
		return float64(v), err
	case TaskMetricPlayGameSecDay:
		gid := FeaturedGameIDForTasks()
		v, err := s.store.GetGameSecDay(ctx, today, accountID, gid)
		return float64(v), err
	case TaskMetricPlayOpensLifetime:
		v, err := s.store.GetPlayOpensLifetime(ctx, accountID)
		return float64(v), err
	case TaskMetricGoldEarnedDay:
		v, err := s.store.GetGoldEarnedDay(ctx, today, accountID)
		return v, err
	case TaskMetricGoldEarnedLifetime:
		v, err := s.store.GetGoldEarnedLifetime(ctx, accountID)
		return v, err
	case TaskMetricPageViewDay:
		ok, err := s.store.HasPageViewDay(ctx, today, accountID, def.PageKey)
		if err != nil {
			return 0, err
		}
		if ok {
			return 1, nil
		}
		return 0, nil
	case TaskMetricShareSuccessDay:
		ok, err := s.store.HasShareSuccessDay(ctx, today, accountID)
		if err != nil {
			return 0, err
		}
		if ok {
			return 1, nil
		}
		return 0, nil
	case TaskMetricProfileComplete:
		if uc.ProfileComplete {
			return 1, nil
		}
		return 0, nil
	case TaskMetricGuestUpgraded:
		if uc.GuestUpgraded {
			return 1, nil
		}
		return 0, nil
	case TaskMetricAccountAgeDays:
		return float64(uc.AccountAgeDays), nil
	case TaskMetricAccountIdleDays:
		if uc.AccountIdleDays >= def.MinIdleDays {
			return float64(uc.AccountIdleDays), nil
		}
		return float64(uc.AccountIdleDays), nil
	case TaskMetricInviteTotal:
		return float64(uc.InviteTotal), nil
	case TaskMetricHasInviter:
		if uc.HasInviter {
			return 1, nil
		}
		return 0, nil
	case TaskMetricAdsCompleteWeek:
		v, err := s.store.GetAdsCompleteWeek(ctx, weekID, accountID)
		return float64(v), err
	case TaskMetricAdsCompleteLifetime:
		v, err := s.store.GetAdsCompleteLifetime(ctx, accountID)
		return float64(v), err
	case TaskMetricSignin7dRounds:
		n, err := s.store.GetSignin7dRounds(ctx, accountID)
		return float64(n), err
	default:
		return 0, nil
	}
}

func (s *TaskService) evalStatus(ctx context.Context, accountID string, def TaskDef, today int, signin Signin7dState, uc TaskUserContext) (TaskStatus, float64, error) {
	ck := s.claimKey(def, accountID, today, &signin)
	claimed, err := s.store.IsClaimed(ctx, ck)
	if err != nil {
		return "", 0, err
	}
	if claimed {
		return TaskStatusClaimed, 0, nil
	}

	if def.TaskID == "signin_7d" {
		if signin.LastClaimYmd == today {
			return TaskStatusClaimed, float64(signin.Chain), nil
		}
		if signin.Chain >= 7 {
			return TaskStatusClaimed, float64(signin.Chain), nil
		}
		return TaskStatusClaimable, float64(signin.Chain), nil
	}

	prog, err := s.metricProgress(ctx, accountID, def, today, signin, uc)
	if err != nil {
		return "", 0, err
	}
	if def.ExactAccountAge >= 0 && def.Metric == TaskMetricAccountAgeDays && def.TaskID == "newbie_day1_welcome" {
		if int(prog) != def.ExactAccountAge {
			return TaskStatusInProgress, prog, nil
		}
	}
	if def.TaskID == "comeback_gift" && uc.AccountIdleDays < def.MinIdleDays {
		return TaskStatusInProgress, prog, nil
	}
	if prog >= def.Target {
		return TaskStatusClaimable, prog, nil
	}
	if prog > 0 {
		return TaskStatusInProgress, prog, nil
	}
	return TaskStatusInProgress, prog, nil
}

func (s *TaskService) rewardForDef(def TaskDef, signin Signin7dState, adBonus bool) float64 {
	if def.TaskID == "signin_7d" {
		idx := signin.Chain
		if idx < 0 || idx >= len(Signin7dGoldRewards) {
			return 0
		}
		base := Signin7dGoldRewards[idx]
		if adBonus && def.AdBonusPercent > 0 {
			return base + math.Floor(base*def.AdBonusPercent)
		}
		return base
	}
	base := def.RewardGold
	if adBonus {
		if def.AdBonusGold > 0 {
			return base + def.AdBonusGold
		}
		if def.AdBonusPercent > 0 {
			return base + math.Floor(base*def.AdBonusPercent)
		}
	}
	return base
}

func adBonusAllowed(def TaskDef) bool {
	return def.AdBonusGold > 0 || def.AdBonusPercent > 0
}

// GetWelfare builds task list for account.
func (s *TaskService) GetWelfare(ctx context.Context, accountID string) (WelfareTasksData, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return WelfareTasksData{}, ErrTaskEmptyAccount
	}
	today := s.todayYmd()
	signin, err := s.loadSignin(ctx, accountID)
	if err != nil {
		return WelfareTasksData{}, err
	}
	signinDTO, err := s.buildSigninDTO(ctx, accountID)
	if err != nil {
		return WelfareTasksData{}, err
	}
	uc, err := s.loadUserContext(ctx, accountID, today)
	if err != nil {
		return WelfareTasksData{}, err
	}

	var items []WelfareTaskItem
	byCat := make(map[string][]WelfareTaskItem)
	for _, def := range ListActiveTaskDefs() {
		if !taskDefVisible(def, uc, s.tz) {
			continue
		}
		st, prog, err := s.evalStatus(ctx, accountID, def, today, signin, uc)
		if err != nil {
			return WelfareTasksData{}, err
		}
		descKey := strings.TrimSpace(def.TitleKey) + ".desc"
		item := WelfareTaskItem{
			TaskID:     def.TaskID,
			Category:   string(def.Category),
			TitleKey:   def.TitleKey,
			DescKey:    descKey,
			Metric:     string(def.Metric),
			Target:     def.Target,
			Progress:   prog,
			RewardGold: def.RewardGold,
			Status:     st,
			SortOrder:  def.SortOrder,
		}
		if def.AdBonusGold > 0 {
			item.AdBonusGold = def.AdBonusGold
		} else if def.AdBonusPercent > 0 && def.TaskID == "signin_7d" {
			item.AdBonusGold = math.Floor(Signin7dGoldRewards[min(signin.Chain, 6)] * def.AdBonusPercent)
		}
		items = append(items, item)
		byCat[string(def.Category)] = append(byCat[string(def.Category)], item)
	}
	return WelfareTasksData{
		TodayYmd:   today,
		TierPolicy: GetTaskTierPolicy(),
		Signin7d:   signinDTO,
		Tasks:      items,
		ByCategory: byCat,
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Claim grants reward for taskId.
func (s *TaskService) Claim(ctx context.Context, accountID, taskID string, adBonus bool) (TaskClaimResult, error) {
	accountID = strings.TrimSpace(accountID)
	taskID = strings.TrimSpace(taskID)
	if accountID == "" {
		return TaskClaimResult{}, ErrTaskEmptyAccount
	}
	def, ok := taskDefByID(taskID)
	if !ok || !def.Enabled || !taskVisibleByTier(def.Tier) {
		return TaskClaimResult{}, ErrTaskNotFound
	}
	if s.ledger == nil {
		return TaskClaimResult{}, ErrTaskNoEconomy
	}

	today := s.todayYmd()
	signin, err := s.loadSignin(ctx, accountID)
	if err != nil {
		return TaskClaimResult{}, err
	}

	uc, err := s.loadUserContext(ctx, accountID, today)
	if err != nil {
		return TaskClaimResult{}, err
	}
	if !taskDefVisible(def, uc, s.tz) {
		return TaskClaimResult{}, ErrTaskNotFound
	}
	st, _, err := s.evalStatus(ctx, accountID, def, today, signin, uc)
	if err != nil {
		return TaskClaimResult{}, err
	}
	if st != TaskStatusClaimable {
		if st == TaskStatusClaimed {
			return TaskClaimResult{}, ErrTaskAlreadyClaimed
		}
		return TaskClaimResult{}, ErrTaskNotClaimable
	}

	if adBonus && adBonusAllowed(def) {
		okProof, err := s.store.HasAdProof(ctx, accountID, time.Now())
		if err != nil {
			return TaskClaimResult{}, err
		}
		if !okProof {
			return TaskClaimResult{}, ErrTaskAdProofInvalid
		}
	}

	amount := s.rewardForDef(def, signin, adBonus)
	if amount <= 0 {
		return TaskClaimResult{}, ErrTaskNotClaimable
	}

	bizNo := fmt.Sprintf("%s:%d:%s", taskID, today, accountID)
	if def.TaskID == "signin_7d" {
		bizNo = fmt.Sprintf("signin_7d:%d:%s", today, accountID)
	}

	res, err := s.ledger.ApplyGold(ctx, accountID, GoldOpAdd, amount, GoldApplyOpts{
		BizType: def.BizType,
		BizNo:   bizNo,
		Reason:  "welfare_task:" + taskID,
	})
	if err != nil {
		return TaskClaimResult{}, err
	}

	ck := s.claimKey(def, accountID, today, &signin)
	if err := s.store.MarkClaimed(ctx, ck); err != nil {
		return TaskClaimResult{}, err
	}

	if res.GrantedDelta > 0 {
		_, _ = s.store.IncrGoldEarnedDay(ctx, today, accountID, res.GrantedDelta)
		_, _ = s.store.IncrGoldEarnedLifetime(ctx, accountID, res.GrantedDelta)
	}

	if def.TaskID == "signin_7d" {
		if signin.Chain == 0 && signin.RoundStartYmd == 0 {
			signin.RoundStartYmd = today
		}
		if signin.Chain == 0 {
			signin.RoundStartYmd = today
		}
		signin.Chain++
		signin.LastClaimYmd = today
		if err := s.store.SetSignin7d(ctx, accountID, signin); err != nil {
			return TaskClaimResult{}, err
		}
		_ = s.store.RecordSigninClaimForStreak(ctx, today, accountID)
		if signin.Chain >= 7 {
			_, _ = s.store.IncrSignin7dRounds(ctx, accountID)
		}
	}

	signinDTO, _ := s.buildSigninDTO(ctx, accountID)
	var signinPtr *Signin7dDTO
	if def.TaskID == "signin_7d" {
		signinPtr = &signinDTO
	}

	return TaskClaimResult{
		TaskID:            taskID,
		GrantedGold:       res.GrantedDelta,
		CurGold:           res.After.CurGold,
		DailyCapRemaining: res.DailyCapRemaining,
		Status:            TaskStatusClaimed,
		Signin7d:          signinPtr,
	}, nil
}

// OnRankActivity records play time progress.
func (s *TaskService) OnRankActivity(ctx context.Context, accountID, gameID string, durationSec int64) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" || durationSec <= 0 {
		return nil
	}
	today := s.todayYmd()
	weekID := CurrentActivityWeekID(GoldNow(s.tz))
	total, err := s.store.IncrActiveSecDay(ctx, today, accountID, durationSec)
	if err != nil {
		return err
	}
	_, _ = s.store.IncrActiveSecWeek(ctx, weekID, accountID, durationSec)
	_, _ = s.store.IncrActiveSecLifetime(ctx, accountID, durationSec)
	gameID = strings.TrimSpace(gameID)
	if gameID != "" {
		_, _ = s.store.AddGameSecDay(ctx, today, accountID, gameID, durationSec)
	}
	if total >= 60 || (total-durationSec < 60 && total >= 60) {
		_ = s.store.RecordActiveDayForStreak(ctx, today, accountID)
	}
	return nil
}

// OnRankPlay records a play open for lifetime play tasks.
func (s *TaskService) OnRankPlay(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil
	}
	_, err := s.store.IncrPlayOpensLifetime(ctx, accountID)
	return err
}

// ReportPageView records daily page visit (welfare/rank).
func (s *TaskService) ReportPageView(ctx context.Context, accountID, page string) error {
	page = strings.TrimSpace(strings.ToLower(page))
	accountID = strings.TrimSpace(accountID)
	if accountID == "" || page == "" {
		return nil
	}
	return s.store.MarkPageViewDay(ctx, s.todayYmd(), accountID, page)
}

// ReportShare records daily share success.
func (s *TaskService) ReportShare(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil
	}
	return s.store.MarkShareSuccessDay(ctx, s.todayYmd(), accountID)
}

// OnAdsComplete records ad count and ad proof for bonus claims.
func (s *TaskService) OnAdsComplete(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil
	}
	today := s.todayYmd()
	weekID := CurrentActivityWeekID(GoldNow(s.tz))
	if _, err := s.store.IncrAdsCompleteDay(ctx, today, accountID); err != nil {
		return err
	}
	_, _ = s.store.IncrAdsCompleteWeek(ctx, weekID, accountID)
	_, _ = s.store.IncrAdsCompleteLifetime(ctx, accountID)
	return s.store.SetAdProof(ctx, accountID, time.Now().Add(taskAdProofTTL))
}

// OnInviteRegistered increments inviter's daily invite count.
func (s *TaskService) OnInviteRegistered(ctx context.Context, inviterAccountID string) error {
	inviterAccountID = strings.TrimSpace(inviterAccountID)
	if inviterAccountID == "" {
		return nil
	}
	_, err := s.store.IncrInviteValidDay(ctx, s.todayYmd(), inviterAccountID)
	return err
}
