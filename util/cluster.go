package util

import "os"

var inCluster bool

func SetCluster(test func() bool) {
	inCluster = test()
}

func InCluster() bool {
	return inCluster
}

func IsFlyCluster() bool {
	_, ok := os.LookupEnv("FLY_APP_NAME")

	return ok
}
