package regpolicy

// RegistrationShouldDisableAdRewards 判断即将插入的账号是否应打上「不发激励广告奖励」标记。
//
// existingDeviceRegsToday：同一 natural 日下同 device_id（非空）已注册账号数量（不含当前即将插入）。
// existingIPRegsToday：同 registration_ip（非空且可信）已注册账号数量。
// devLimit/ipLimit：任一 <=0 表示不启用该维度；>0 当且仅当当日该维度计数 >= limit 时对「下一笔」注册关奖励。
//
// 「一机一号」放宽二手手机：devLimit 默认可设 2（前两个账号仍可领奖，第三个起关奖励）。
func RegistrationShouldDisableAdRewards(existingDeviceRegsToday, existingIPRegsToday int, deviceLimitPerDay, ipLimitPerDay int, hasDeviceID, evaluateIPDimension bool) bool {
	devViolates := deviceLimitPerDay > 0 && hasDeviceID && existingDeviceRegsToday >= deviceLimitPerDay
	ipViolates := ipLimitPerDay > 0 && evaluateIPDimension && existingIPRegsToday >= ipLimitPerDay
	return devViolates || ipViolates
}
