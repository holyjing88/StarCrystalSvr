package service

import (
	"encoding/json"
	"strings"
)

// gameChannelsJSON 支持 JSON 中为字符串或字符串数组。
// 字符串内多个渠道用 ASCII 连字符 "-" 连接（如 ChannelType_A-ChannelType_B），表示游戏可出现在其中任一渠道。
type gameChannelsJSON []string

func (c *gameChannelsJSON) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*c = nil
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*c = splitGameConfigChannels(s)
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	var merged []string
	for _, x := range arr {
		merged = append(merged, splitGameConfigChannels(x)...)
	}
	*c = merged
	return nil
}

// SplitGameConfigChannels 将配置里的 channels 字段拆成多个渠道类型 token（仅 "-" 为分隔符）。
func SplitGameConfigChannels(s string) []string {
	return splitGameConfigChannels(s)
}

func splitGameConfigChannels(s string) []string {
	parts := strings.Split(strings.TrimSpace(s), "-")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// CanonicalGameChannelKey 将 token 规范为统一键以便匹配；兼容历史 CompanyOwned / GooglePlay。
func CanonicalGameChannelKey(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	switch {
	case strings.EqualFold(v, "GooglePlay"), strings.EqualFold(v, "ChannelType_GooglePlay"):
		return "ChannelType_GooglePlay"
	case strings.EqualFold(v, "CompanyOwned"), strings.EqualFold(v, "ChannelType_CompanyOwned"):
		return "ChannelType_CompanyOwned"
	default:
		return v
	}
}

func canonicalGameChannelSet(tokens []string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, t := range tokens {
		k := CanonicalGameChannelKey(t)
		if k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}

// gameChannelSingleClientMatch：客户端只传一个渠道（不做拆包）。配置的 channels 已拆成 gameTokens。
// 配置为空：全渠道可见。
// 配置非空：须与客户端 channel 规范化后命中其一；未传 channel 则不返回该游戏（避免未带渠道的请求拿到渠道独占配置）。
func gameChannelSingleClientMatch(clientChannel string, gameTokens []string) bool {
	gameSet := canonicalGameChannelSet(gameTokens)
	if len(gameSet) == 0 {
		return true
	}
	cc := strings.TrimSpace(clientChannel)
	if cc == "" {
		return false
	}
	k := CanonicalGameChannelKey(cc)
	if k == "" {
		return false
	}
	_, ok := gameSet[k]
	return ok
}
