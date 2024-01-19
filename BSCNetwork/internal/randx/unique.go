package randx

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"time"
)

const orderIdLen = len("23011223353924499737")

// GenUniqueId 生成订单号
func GenUniqueId() string {
	now := time.Now()
	sn := fmt.Sprintf("%s%03d%04d", now.Format("060102150405"), now.Nanosecond()/1e6, RandInt(9999))

	// hash str to 0 - 9
	new32 := fnv.New32()
	new32.Write([]byte(sn))
	return fmt.Sprintf("%s%d", sn, new32.Sum32()%10)
}

func RandInt(number int) int {
	randInt := rand.New(rand.NewSource(time.Now().UnixNano()))
	return randInt.Intn(number)
}
