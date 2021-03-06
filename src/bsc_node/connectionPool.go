package bsc_node

import (
	"context"
	"github.com/ethereum/go-ethereum/ethclient"
	pool "github.com/jolestar/go-commons-pool/v2"
	log "github.com/sirupsen/logrus"
	"sampleGoEthereum/src/app"
)

type NodeConnection struct {
	addresses []string
	index     int
}

func NodeConnectionFactory(addressList []string) NodeConnection {
	return NodeConnection{
		addresses: addressList,
		index:     0,
	}
}

func (connFactory *NodeConnection) clientFactory(ctx context.Context) *NodeClient {
	if connFactory.index >= len(connFactory.addresses) {
		connFactory.index = 0
	}

	client, err := ethclient.DialContext(ctx, connFactory.addresses[connFactory.index])
	if err != nil {
		log.Fatal("ethclient.Dial failed", err)
	}

	log.Debugf("connecting to %s success . . . ", connFactory.addresses[connFactory.index])
	node := &NodeClient{
		connFactory.addresses[connFactory.index],
		client,
	}
	connFactory.index++
	return node
}



func CreateConnectionPool(ctx context.Context, connectionFactory NodeConnection) *pool.ObjectPool {
	factory := pool.NewPooledObjectFactorySimple(
		func(context.Context) (interface{}, error) {
			return connectionFactory.clientFactory(ctx), nil
		})

	config := &pool.ObjectPoolConfig {
		MaxTotal:                 app.Globals.Config.ConnectionPoolMaxTotal,
		MaxIdle:                  app.Globals.Config.ConnectionPoolMaxIDLE,
		MinIdle:                  app.Globals.Config.ConnectionPoolMinIDLE,
	}

	connectionPool := pool.NewObjectPool(ctx, factory, config)
	connectionPool.PreparePool(ctx)
	return connectionPool
}