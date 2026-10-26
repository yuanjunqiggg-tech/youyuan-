package building

import "time"

// -1 与 0 代表不加人为延时；正数仍允许用户设置自己的运行速率。
func UnlimitedSpeedDelay(speed int) time.Duration {
	if speed <= 0 {
		return 0
	}
	return time.Second / time.Duration(speed)
}
func yySpeedDelay(speed int) time.Duration { return UnlimitedSpeedDelay(speed) }
