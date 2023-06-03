package session

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/go-logr/logr"
	"github.com/luthermonson/go-proxmox"
	"github.com/pkg/errors"
	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/constants"
	ctrl "sigs.k8s.io/controller-runtime"
)

const ProviderUserAgent = "k8s-cappx-useragent"

var (
	// global Session map against sessionKeys in map[sessionKey]Session.
	sessionCache sync.Map

	// mutex to control access to the GetOrCreate function to avoid duplicate
	// session creations on startup.
	sessionMU sync.Mutex
)

// Session is a Proxmox session with a configured Cluster.
type Session struct {
	*proxmox.Client
	ProxmoxCluster *proxmox.Cluster
	VMs            map[string]*proxmox.VirtualMachine
	Nodes          map[string]*proxmox.Node
}

type Feature struct {
	EnableKeepAlive   bool
	KeepAliveDuration time.Duration
}

func DefaultFeature() Feature {
	return Feature{
		EnableKeepAlive: constants.DefaultEnableKeepAlive,
	}
}

type Params struct {
	server     string
	datacenter string
	cluster    string
	token      string
	ticket     string
	userinfo   *url.Userinfo
	thumbprint string
	feature    Feature
}

func NewParams() *Params {
	return &Params{
		feature: DefaultFeature(),
	}
}

func (p *Params) WithServer(server string) *Params {
	p.server = server
	return p
}

func (p *Params) WithDatacenter(datacenter string) *Params {
	p.datacenter = datacenter
	return p
}

func (p *Params) WithCluster(cluster string) *Params {
	p.cluster = cluster
	return p
}

func (p *Params) WithUserInfo(username, password string) *Params {
	p.userinfo = url.UserPassword(username, password)
	return p
}

func (p *Params) WithThumbprint(thumbprint string) *Params {
	p.thumbprint = thumbprint
	return p
}

func (p *Params) WithFeatures(feature Feature) *Params {
	p.feature = feature
	return p
}

// GetOrCreate gets a cached session or creates a new one if one does not
// already exist.
func GetOrCreate(ctx context.Context, params *Params) (*Session, error) {
	logger := ctrl.LoggerFrom(ctx).WithName("session")
	sessionMU.Lock()
	defer sessionMU.Unlock()

	sessionKey := path.Join(params.server, params.userinfo.Username(), params.datacenter)
	// TODO implement proxmox client session caching
	//if cachedSession, ok := sessionCache.Load(sessionKey); ok {
	//	s := cachedSession.(*Session)
	//	logger = logger.WithValues("server", params.server, "cluster", params.cluster)
	//}

	clearCache(logger, sessionKey)

	serverPath := params.server
	if !strings.Contains(serverPath, "https") {
		newPath, err := url.JoinPath("https://", serverPath)
		if err != nil {
			return nil, err
		}
		serverPath = newPath
	}
	proxmoxUrl, err := url.Parse(serverPath)
	if err != nil {
		return nil, errors.Errorf("error parsing Proxmox server URL from %q", serverPath)
	}
	client, err := newClient(ctx, logger, sessionKey, proxmoxUrl, params)
	if err != nil {
		return nil, err
	}

	session := Session{Client: client}
	// TODO add user agent field to go-proxmox client

	// Cache the session.
	sessionCache.Store(sessionKey, &session)

	logger.V(2).Info("cached Proxmox client session", "server", params.server, "cluster", params.cluster)

	return &session, nil
}

func newClient(ctx context.Context, logger logr.Logger, sessionKey string, url *url.URL, params *Params) (*proxmox.Client, error) {
	pw, ok := params.userinfo.Password()
	if !ok {
		return nil, errors.Errorf("empty password supplied for proxmox user %s", params.userinfo.Username())
	}
	hclient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 10 * time.Second,
	}

	client := proxmox.NewClient(fmt.Sprintf("%s://%s%s", url.Scheme, url.Host, url.Path),
		proxmox.WithUserAgent(ProviderUserAgent),
		proxmox.WithClient(&hclient),
		proxmox.WithLogins(params.userinfo.Username(), pw),
	)
	if client == nil {
		return nil, errors.Errorf("error creating Proxmox client for %q", url.Host)
	}
	creds := proxmox.Credentials{}
	creds.Username = params.userinfo.Username()
	creds.Password = pw
	creds.Realm = "pam"
	session, err := client.Ticket(&creds)
	if err != nil {
		return nil, err
	}

	return proxmox.NewClient(fmt.Sprintf("%s://%s%s", url.Scheme, url.Host, url.Path),
		proxmox.WithUserAgent(ProviderUserAgent),
		proxmox.WithClient(&hclient),
		proxmox.WithLogins(params.userinfo.Username(), pw),
		proxmox.WithSession(session.Ticket, session.CsrfPreventionToken),
	), nil
	// TODO implement custom CA support for go-proxmox client
	//insecure := thumbprint == ""
	//if !insecure {
	//	client.SetThumbprint(url.Host, thumbprint)
	//}

	//pw, ok := url.User.Password()

	//if err := client.Login(params.userinfo.Username(), session.Ticket); err != nil {
	//	return nil, err
	//}

	//return client, nil
}

func clearCache(logger logr.Logger, sessionKey string) {
	//if cachedSession, ok := sessionCache.Load(sessionKey); ok {
	//	s := cachedSession.(*Session)
	//logger.Info("performing session log out and clearing session", "key", sessionKey)
	// TODO implement logout for go-proxmox client
	//if err := s.Logout(context.Background()); err != nil {
	//	logger.Error(err, "unable to logout session")
	//}
	//}
	sessionCache.Delete(sessionKey)
}

func (s *Session) GetVersion() (infrav1.ProxmoxVersion, error) {
	svcVersion, err := s.Version()
	if err != nil {
		return "", err
	}
	if svcVersion == nil {
		return infrav1.NewProxmoxVersion("7"), nil
	}
	version, err := semver.New(strings.Replace(svcVersion.Version, "-", ".", 1))
	if err != nil {
		return "", err
	}

	switch version.Major {
	case 6, 7, 8:
		return infrav1.NewProxmoxVersion(version.String()), nil
	default:
		return "", unidentifiedProxmoxVersion{version: svcVersion.Version}
	}
}

// Clear is meant to destroy all the cached sessions.
func Clear() {
	sessionCache.Range(func(key, s any) bool {
		// TODO implement logout
		//cachedSession := s.(*Session)
		//_ = cachedSession.Logout(context.Background())
		return true
	})
}
