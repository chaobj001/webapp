package session

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

//设计思路来源于database/sql/driver，先定义好接口，然后具体的存储session的结构实现相应的接口并注册后，相应功能这样就可以使用了，以下是用来随需注册存储session的结构的Register函数的实现。
type Session interface {
	Set(key, value interface{}) error
	Get(key interface{}) interface{}
	Delete(key interface{}) error
	SessionID() string
}

//抽象出一个Provider接口，用以表征session管理器底层存储结构
type Provider interface {
	SessionInit(sid string) (Session, error) //SessionInit函数实现Session的初始化，操作成功则返回此新的Session变量
	SessionRead(sid string) (Session, error) //回sid所代表的Session变量，如果不存在，那么将以sid为参数调用SessionInit函数创建并返回一个新的Session变量
	SessionDestory(sid string) error         //用来销毁sid对应的Session变量
	SessionGC(maxLifeTime int64)             //根据maxLifeTime来删除过期的数据
}

var provides = make(map[string]Provider)

func Register(name string, provider Provider) {
	if provider == nil {
		panic("seesion: Register provide is nil")
	}
	if _, dup := provides[name]; dup {
		panic("session: Register called twice for provide " + name)
	}
	provides[name] = provider
}

type Manager struct {
	cookieName  string
	lock        sync.Mutex
	provider    Provider
	maxlifetime int64
}

//全局唯一的Session ID
func (manager *Manager) sessionId() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

//session创建
func (manager *Manager) SessionStart(w http.ResponseWriter, r *http.Request) (session Session) {
	manager.lock.Lock()
	defer manager.lock.Unlock()
	cookie, err := r.Cookie(manager.cookieName)
	if err != nil || cookie.Value == "" {
		sid := manager.sessionId()
		session, _ = manager.provider.SessionInit(sid)
		cookie := http.Cookie{Name: manager.cookieName, Value: url.QueryEscape(sid), Path: "/", HttpOnly: true, MaxAge: int(manager.maxlifetime)}
		http.SetCookie(w, &cookie)
	} else {
		sid, _ := url.QueryUnescape(cookie.Value)
		session, _ = manager.provider.SessionRead(sid)
	}
	return
}
func (manager *Manager) SessionDestory(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(manager.cookieName)
	if err != nil || cookie.Value == "" {
		return
	} else {
		manager.lock.Lock()
		defer manager.lock.Unlock()
		manager.provider.SessionDestory(cookie.Value)
		expiration := time.Now()
		cookie := http.Cookie{Name: manager.cookieName, Path: "/", HttpOnly: true, Expires: expiration, MaxAge: -1}
		http.SetCookie(w, &cookie)
	}
}

//session销毁
func (manager *Manager) GC() {
	manager.lock.Lock()
	defer manager.lock.Unlock()
	manager.provider.SessionGC(manager.maxlifetime)
	time.AfterFunc(time.Duration(manager.maxlifetime), func() { manager.GC() })
}

func NewManager(provideName, cookieName string, maxlifetime int64) (*Manager, error) {
	provider, ok := provides[provideName]
	if !ok {
		return nil, fmt.Errorf("session unknown provide %q (forgotten import?)", provideName)
	}
	return &Manager{provider: provider, cookieName: cookieName, maxlifetime: maxlifetime}, nil
}
