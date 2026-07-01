package service

// GameChannelsPatch 供 IDIP upsert 合并 channels 字段（自 D:\...\server 同步；原定义在 game_channels.go）。
type GameChannelsPatch gameChannelsJSON

func (c *GameChannelsPatch) UnmarshalJSON(data []byte) error {
	var inner gameChannelsJSON
	if err := (&inner).UnmarshalJSON(data); err != nil {
		return err
	}
	*c = GameChannelsPatch(inner)
	return nil
}
