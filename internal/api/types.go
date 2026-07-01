package api

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type GameItem struct {
	GameID        string `json:"gameId"`
	Name          string `json:"name"`
	NameEn        string `json:"nameEn,omitempty"`
	NameUr        string `json:"nameUr,omitempty"`
	Note          string `json:"note,omitempty"`
	NoteEn        string `json:"noteEn,omitempty"`
	NoteUr        string `json:"noteUr,omitempty"`
	IconLink      string `json:"iconLink,omitempty"`
	CoverURL      string `json:"coverUrl,omitempty"`
	EntryType     string `json:"entryType"`
	EntryURL       string `json:"entryUrl"`
	DownloadURL    string `json:"downloadUrl,omitempty"`
	PackageBytes   int64  `json:"packageBytes,omitempty"`
	DownloadSha256 string `json:"downloadSha256,omitempty"`
	MinAppVersion  string `json:"minAppVersion,omitempty"`
	Sort          int    `json:"sort"`
	RewardRuleID  string `json:"rewardRuleId,omitempty"`
	// 已登录且该游戏在账号收藏集合内时为 true（GET /api/v1/games + Bearer）。
	Favorited bool `json:"favorited,omitempty"`
}

// GameFavoritesListData GET /api/v1/games/favorites
type GameFavoritesListData struct {
	GameIDs []string `json:"gameIds"`
}

// GameFavoriteToggleData POST/DELETE /api/v1/games/favorite
type GameFavoriteToggleData struct {
	GameID    string `json:"gameId"`
	Favorited bool   `json:"favorited"`
}

type GameListResponseData struct {
	ConfigVersion string     `json:"configVersion"`
	ServerTime    string     `json:"serverTime"`
	Games         []GameItem `json:"games"`
}

type WalletBalance struct {
	Balance      float64 `json:"balance"`
	FrozenAmount float64 `json:"frozenAmount"`
	TotalIncome  float64 `json:"totalIncome"`
	TotalGiftRedeem float64 `json:"totalGiftRedeem"`
}

type WalletLedgerItem struct {
	LedgerNo  string  `json:"ledgerNo"`
	BizType   string  `json:"bizType"`
	BizNo     string  `json:"bizNo"`
	Amount    float64 `json:"amount"`
	Direction string  `json:"direction"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"createdAt"`
}

// RankListResponseData GET /api/v1/rank
type RankListResponseData struct {
	Board    string         `json:"board"`
	Period   string         `json:"period,omitempty"`
	WeekID   string         `json:"weekId,omitempty"`
	Items    []RankListItem `json:"items"`
	MyRank   int64          `json:"myRank,omitempty"`
	MyScore  float64        `json:"myScore,omitempty"`
}

// RankListItem 单条排行（人气：playCount；活跃：activeScore；福利：gold / token / accountId）。
type RankListItem struct {
	GameID      string  `json:"gameId,omitempty"`
	AccountID   string  `json:"accountId,omitempty"`
	Name        string  `json:"name"`
	PlayCount   int64   `json:"playCount,omitempty"`
	ActiveScore int64   `json:"activeScore,omitempty"`
	Gold        float64 `json:"gold,omitempty"`
	Token       float64 `json:"token,omitempty"`
	Score       float64 `json:"score,omitempty"`
	Rank        int64   `json:"rank,omitempty"`
}

// RankPlayResponseData POST /api/v1/rank/play 返回当前累计次数。
type RankPlayResponseData struct {
	GameID    string `json:"gameId"`
	PlayCount int64  `json:"playCount"`
}

// RankActivityResponseData POST /api/v1/rank/activity 返回本周累计活跃秒数（按账号）。
type RankActivityResponseData struct {
	AccountID   string `json:"accountId"`
	GameID      string `json:"gameId,omitempty"`
	ActiveScore int64  `json:"activeScore"`
	WeekID      string `json:"weekId"`
}
