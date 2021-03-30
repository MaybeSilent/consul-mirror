package lib

import (
	crand "crypto/rand"
	"math"
	"math/big"
	"math/rand"
	"sync"
	"time"
)
// 随机数生成
var (
	once sync.Once

	// SeededSecurely is set to true if a cryptographically secure seed
	// was used to initialize rand.  When false, the start time is used
	// as a seed.
	SeededSecurely bool
)

//
// SeedMathRand provides weak, but guaranteed seeding, which is better than
// running with Go's default seed of 1.  A call to SeedMathRand() is expected
// to be called via init(), but never a second time.
func SeedMathRand() {
	once.Do(func() { // 单例
		n, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			// 使用时间作为随机数的种子
			rand.Seed(time.Now().UTC().UnixNano())
			return
		}
		rand.Seed(n.Int64()) // 设置随机种子
		SeededSecurely = true // 使用 cryptographically secure seed （密码学上安全的种子） 设置为true
	})
}

// - 计算机产生的随机数都是有规律的伪随机数
// - 随机种子来自系统时钟等，计算机主板上的定时/计数器在内存中的记数值
// - 随机数是由随机种子根据一定的计算方法计算出来的数值。所以，只要计算方法一定，随机种子一定，那么产生的随机数就不会变
// - 如果想在一个程序中生成随机数序列，需要至多在生成随机数之前设置一次随机种子