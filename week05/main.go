type (
	// RollingWindowOption let callers customize the RollingWindow.
	RollingWindowOption func(rollingWindow *RollingWindow)

	// RollingWindow defines a rolling window to calculate the events in buckets with time interval.
	RollingWindow struct {
		lock          sync.RWMutex
		// bucket 数量
		size          int
		// 存储 bucket, 环形数组 offset % size 将操作映射到范围内
		win           *window
		// 每个桶时间间隔
		interval      time.Duration
		// 上一个 add 时桶的偏移量
		offset        int
		// reduce 取数据时是否忽略当前还未结束的桶
		ignoreCurrent bool
		// 上次 add 时的时间
		lastTime      time.Duration // start time of the last bucket
	}
)

// Add adds value to current bucket.
func (rw *RollingWindow) Add(v float64) {
	rw.lock.Lock()
	defer rw.lock.Unlock()
	// 这里处理偏移量
	// 1. reset 掉过期的桶
	// 2. 计算当前偏移量 rw.offset
	// 3. 更新 rw.lastTime
	rw.updateOffset()
	// 使用上一步算好的 offset
	rw.win.add(rw.offset, v)
}

// Reduce runs fn on all buckets, ignore current bucket if ignoreCurrent was set.
func (rw *RollingWindow) Reduce(fn func(b *Bucket)) {
	rw.lock.RLock()
	defer rw.lock.RUnlock()

	// 由于 reset 过期桶操作只在 add 中的 updateOffset 中调用
	// Reduce 读取时不做 reset 操作, 但是只返回还没过期的桶
	var diff int
	// span 函数返回当前时间距离上次 add 时间过了几个 interval
	// 也就是过期几个桶
	span := rw.span()
	// rw.ignoreCurrent 为 true 时, 忽略当前桶
	if span == 0 && rw.ignoreCurrent {
		diff = rw.size - 1
	} else {
		// size - span 表示还未过期的桶, 也就是要取的数据
		diff = rw.size - span
	}
	// <= 0 时表示都过期了
	if diff > 0 {
		// 过期的桶为 [rw.offset+1, rw.offset+span], diff 为没过期的桶数量
		offset := (rw.offset + span + 1) % rw.size
		// 所以从 rw.offset+span+1 开始拿 diff 个桶
		rw.win.reduce(offset, diff, fn)
	}
}

// span 函数返回当前时间距离上次 add 时间过了几个 interval
// 也就是过期几个桶
func (rw *RollingWindow) span() int {
	offset := int(timex.Since(rw.lastTime) / rw.interval)
	if 0 <= offset && offset < rw.size {
		return offset
	}

	// offset >= rw.size 表示已经过了环形数组一圈了, 返回 size
	return rw.size
}

func (rw *RollingWindow) updateOffset() {
	// span 返回距离上次 add 过了几个 interval
	span := rw.span()
	if span <= 0 {
		return
	}

	// 经过了span 个 interval, 就说明了 span 个桶已经过期, 需要重置
	// offset 指向上次 add 的 bucket
	offset := rw.offset
	// 由于是环形数组, 所以自己前面是最老的数据
	// 因此从 offset + 1 开始 reset span 个桶
	// [rw.offset+1, rw.offset+span]
	for i := 0; i < span; i++ {
		rw.win.resetBucket((offset + i + 1) % rw.size)
	}

	// 计算出当前的偏移量, 后面会使用它来向相应桶添加数据
	// 也会用作下次 add 计算过期桶
	rw.offset = (offset + span) % rw.size
	now := timex.Now()
	// 更新上次更新时间
	rw.lastTime = now - (now-rw.lastTime)%rw.interval
}
