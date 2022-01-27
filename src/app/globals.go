package app

import (
	pool "github.com/jolestar/go-commons-pool/v2"
	"sampleGoEthereum/src/configs"
)

var Globals struct {
	ConnectionPool 			  *pool.ObjectPool
	Config                    *configs.Config
}
