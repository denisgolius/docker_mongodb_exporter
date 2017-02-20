package shared

import (
	"sync"
	"time"

	"github.com/golang/glog"
	"gopkg.in/mgo.v2"
)

const (
	dialMongodbTimeout = 10 * time.Second
	syncMongodbTimeout = 1 * time.Minute
)

type Connection struct {
	sync.Mutex
	Uri     string
	Session *mgo.Session
}

func NewConnection(uri string) *Connection {
	return &Connection{
		Uri:     uri,
		Session: newMongoSession(uri),
	}
}

func (c *Connection) Close() {
	if c.Session != nil {
		c.Session.Close()
	}
}

func (c *Connection) GetSession() *mgo.Session {
	if c.Session != nil {
		c.Lock()
		defer c.Unlock()
		return c.Session.Copy()
	}
	return nil
}

func newMongoSession(uri string) *mgo.Session {
	dialInfo, err := mgo.ParseURL(uri)
	if err != nil {
		glog.Errorf("Cannot connect to server using url %s: %s", uri, err)
		return nil
	}

	dialInfo.Direct = true // Force direct connection
	dialInfo.Timeout = dialMongodbTimeout

	session, err := mgo.DialWithInfo(dialInfo)
	if err != nil {
		glog.Errorf("Cannot connect to server using url %s: %s", uri, err)
		return nil
	}
	session.SetMode(mgo.Eventual, true)
	session.SetSyncTimeout(syncMongodbTimeout)
	session.SetSocketTimeout(0)
	return session
}

func (c *Connection) ServerVersion() (string, error) {
	buildInfo, err := c.Session.BuildInfo()
	if err != nil {
		glog.Errorf("Could not get MongoDB BuildInfo: %s!", err)
		return "unknown", err
	}
	return buildInfo.Version, nil
}

func (c *Connection) NodeType() (string, error) {
	masterDoc := struct {
		SetName interface{} `bson:"setName"`
		Hosts   interface{} `bson:"hosts"`
		Msg     string      `bson:"msg"`
	}{}
	err := c.Session.Run("isMaster", &masterDoc)
	if err != nil {
		glog.Errorf("Got unknown node type: %s", err)
		return "unknown", err
	}

	if masterDoc.SetName != nil || masterDoc.Hosts != nil {
		return "replset", nil
	} else if masterDoc.Msg == "isdbgrid" {
		// isdbgrid is always the msg value when calling isMaster on a mongos
		// see http://docs.mongodb.org/manual/core/sharded-cluster-query-router/
		return "mongos", nil
	}
	return "mongod", nil
}
