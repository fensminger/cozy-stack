package middlewares

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cozy/cozy-stack/pkg/instance"
	"github.com/cozy/echo"
)

type (
	// XFrameOption type for the values of the X-Frame-Options header.
	XFrameOption string

	// CSPSource type are the different types of CSP headers sources definitions.
	// Each source type defines a different acess policy.
	CSPSource int

	// SecureConfig defines the config for Secure middleware.
	SecureConfig struct {
		HSTSMaxAge     time.Duration
		CSPDefaultSrc  []CSPSource
		CSPScriptSrc   []CSPSource
		CSPFrameSrc    []CSPSource
		CSPConnectSrc  []CSPSource
		CSPFontSrc     []CSPSource
		CSPImgSrc      []CSPSource
		CSPManifestSrc []CSPSource
		CSPMediaSrc    []CSPSource
		CSPObjectSrc   []CSPSource
		CSPStyleSrc    []CSPSource
		CSPWorkerSrc   []CSPSource

		CSPDefaultSrcWhitelist  string
		CSPScriptSrcWhitelist   string
		CSPFrameSrcWhitelist    string
		CSPConnectSrcWhitelist  string
		CSPFontSrcWhitelist     string
		CSPImgSrcWhitelist      string
		CSPManifestSrcWhitelist string
		CSPMediaSrcWhitelist    string
		CSPObjectSrcWhitelist   string
		CSPStyleSrcWhitelist    string
		CSPWorkerSrcWhitelist   string

		XFrameOptions XFrameOption
		XFrameAllowed string
	}
)

const (
	// XFrameDeny is the DENY option of the X-Frame-Options header.
	XFrameDeny XFrameOption = "DENY"
	// XFrameSameOrigin is the SAMEORIGIN option of the X-Frame-Options header.
	XFrameSameOrigin = "SAMEORIGIN"
	// XFrameAllowFrom is the ALLOW-FROM option of the X-Frame-Options header. It
	// should be used along with the XFrameAllowed field of SecureConfig.
	XFrameAllowFrom = "ALLOW-FROM"

	// CSPSrcSelf is the 'self' option of a CSP source.
	CSPSrcSelf CSPSource = iota
	// CSPSrcData is the 'data:' option of a CSP source.
	CSPSrcData
	// CSPSrcBlob is the 'blob:' option of a CSP source.
	CSPSrcBlob
	// CSPSrcParent adds the parent domain as an eligible CSP source.
	CSPSrcParent
	// CSPSrcWS adds the parent domain eligible for websocket.
	CSPSrcWS
	// CSPSrcSiblings adds all the siblings subdomains as eligibles CSP
	// sources.
	CSPSrcSiblings
	// CSPSrcAny is the '*' option. It allows any domain as an eligible source.
	CSPSrcAny
	// CSPUnsafeInline is the  'unsafe-inline' option. It allows to have inline
	// styles or scripts to be injected in the page.
	CSPUnsafeInline
	// CSPWhitelist inserts a whitelist of domains.
	CSPWhitelist
)

// Secure returns a Middlefunc that can be used to define all the necessary
// secure headers. It is configurable with a SecureConfig object.
func Secure(conf *SecureConfig) echo.MiddlewareFunc {
	var hstsHeader string
	if conf.HSTSMaxAge > 0 {
		hstsHeader = fmt.Sprintf("max-age=%.f; includeSubDomains",
			conf.HSTSMaxAge.Seconds())
	}

	var xFrameHeader string
	switch conf.XFrameOptions {
	case XFrameDeny:
		xFrameHeader = string(XFrameDeny)
	case XFrameSameOrigin:
		xFrameHeader = string(XFrameSameOrigin)
	case XFrameAllowFrom:
		xFrameHeader = fmt.Sprintf("%s %s", XFrameAllowFrom, conf.XFrameAllowed)
	}

	conf.CSPDefaultSrc, conf.CSPDefaultSrcWhitelist =
		validCSPList(conf.CSPDefaultSrc, conf.CSPDefaultSrc, conf.CSPDefaultSrcWhitelist)
	conf.CSPScriptSrc, conf.CSPScriptSrcWhitelist =
		validCSPList(conf.CSPScriptSrc, conf.CSPDefaultSrc, conf.CSPScriptSrcWhitelist)
	conf.CSPFrameSrc, conf.CSPFrameSrcWhitelist =
		validCSPList(conf.CSPFrameSrc, conf.CSPDefaultSrc, conf.CSPFrameSrcWhitelist)
	conf.CSPConnectSrc, conf.CSPConnectSrcWhitelist =
		validCSPList(conf.CSPConnectSrc, conf.CSPDefaultSrc, conf.CSPConnectSrcWhitelist)
	conf.CSPFontSrc, conf.CSPFontSrcWhitelist =
		validCSPList(conf.CSPFontSrc, conf.CSPDefaultSrc, conf.CSPFontSrcWhitelist)
	conf.CSPImgSrc, conf.CSPImgSrcWhitelist =
		validCSPList(conf.CSPImgSrc, conf.CSPDefaultSrc, conf.CSPImgSrcWhitelist)
	conf.CSPManifestSrc, conf.CSPManifestSrcWhitelist =
		validCSPList(conf.CSPManifestSrc, conf.CSPDefaultSrc, conf.CSPManifestSrcWhitelist)
	conf.CSPMediaSrc, conf.CSPMediaSrcWhitelist =
		validCSPList(conf.CSPMediaSrc, conf.CSPDefaultSrc, conf.CSPMediaSrcWhitelist)
	conf.CSPObjectSrc, conf.CSPObjectSrcWhitelist =
		validCSPList(conf.CSPObjectSrc, conf.CSPDefaultSrc, conf.CSPObjectSrcWhitelist)
	conf.CSPStyleSrc, conf.CSPStyleSrcWhitelist =
		validCSPList(conf.CSPStyleSrc, conf.CSPDefaultSrc, conf.CSPStyleSrcWhitelist)
	conf.CSPWorkerSrc, conf.CSPWorkerSrcWhitelist =
		validCSPList(conf.CSPWorkerSrc, conf.CSPDefaultSrc, conf.CSPWorkerSrcWhitelist)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			isSecure := true
			if in := c.Get("instance"); in != nil && in.(*instance.Instance).Dev {
				isSecure = false
			}
			h := c.Response().Header()
			if isSecure && hstsHeader != "" {
				h.Set(echo.HeaderStrictTransportSecurity, hstsHeader)
			}
			if xFrameHeader != "" {
				h.Set(echo.HeaderXFrameOptions, xFrameHeader)
			}
			var cspHeader string
			parent, _, siblings := SplitHost(c.Request().Host)
			if len(conf.CSPDefaultSrc) > 0 {
				cspHeader += makeCSPHeader(parent, siblings, "default-src", conf.CSPDefaultSrcWhitelist, conf.CSPDefaultSrc, isSecure)
			}
			if len(conf.CSPScriptSrc) > 0 {
				cspHeader += makeCSPHeader(parent, siblings, "script-src", conf.CSPScriptSrcWhitelist, conf.CSPScriptSrc, isSecure)
			}
			if len(conf.CSPFrameSrc) > 0 {
				cspHeader += makeCSPHeader(parent, siblings, "frame-src", conf.CSPFrameSrcWhitelist, conf.CSPFrameSrc, isSecure)
			}
			if len(conf.CSPConnectSrc) > 0 {
				cspHeader += makeCSPHeader(parent, siblings, "connect-src", conf.CSPConnectSrcWhitelist, conf.CSPConnectSrc, isSecure)
			}
			if len(conf.CSPFontSrc) > 0 {
				cspHeader += makeCSPHeader(parent, siblings, "font-src", conf.CSPFontSrcWhitelist, conf.CSPFontSrc, isSecure)
			}
			if len(conf.CSPImgSrc) > 0 {
				cspHeader += makeCSPHeader(parent, siblings, "img-src", conf.CSPImgSrcWhitelist, conf.CSPImgSrc, isSecure)
			}
			if len(conf.CSPManifestSrc) > 0 {
				cspHeader += makeCSPHeader(parent, siblings, "manifest-src", conf.CSPManifestSrcWhitelist, conf.CSPManifestSrc, isSecure)
			}
			if len(conf.CSPMediaSrc) > 0 {
				cspHeader += makeCSPHeader(parent, siblings, "media-src", conf.CSPMediaSrcWhitelist, conf.CSPMediaSrc, isSecure)
			}
			if len(conf.CSPObjectSrc) > 0 {
				cspHeader += makeCSPHeader(parent, siblings, "object-src", conf.CSPObjectSrcWhitelist, conf.CSPObjectSrc, isSecure)
			}
			if len(conf.CSPStyleSrc) > 0 {
				cspHeader += makeCSPHeader(parent, siblings, "style-src", conf.CSPStyleSrcWhitelist, conf.CSPStyleSrc, isSecure)
			}
			if len(conf.CSPWorkerSrc) > 0 {
				cspHeader += makeCSPHeader(parent, siblings, "worker-src", conf.CSPWorkerSrcWhitelist, conf.CSPWorkerSrc, isSecure)
			}
			if cspHeader != "" {
				h.Set(echo.HeaderContentSecurityPolicy, cspHeader)
			}
			h.Set(echo.HeaderXContentTypeOptions, "nosniff")
			return next(c)
		}
	}
}

func validCSPList(sources, defaults []CSPSource, whitelist string) ([]CSPSource, string) {
	whitelistFields := strings.Fields(whitelist)
	whitelistFilter := whitelistFields[:0]
	for _, s := range whitelistFields {
		u, err := url.Parse(s)
		if err != nil {
			continue
		}
		u.Scheme = "https"
		if u.Path == "" {
			u.Path = "/"
		}
		whitelistFilter = append(whitelistFilter, u.String())
	}

	if len(whitelistFilter) > 0 {
		whitelist = strings.Join(whitelistFilter, " ")
		sources = append(sources, CSPWhitelist)
	} else {
		whitelist = ""
	}

	if len(sources) == 0 && whitelist == "" {
		return nil, ""
	}

	sources = append(sources, defaults...)
	sourcesUnique := sources[:0]
	for _, source := range sources {
		var found bool
		for _, s := range sourcesUnique {
			if s == source {
				found = true
				break
			}
		}
		if !found {
			sourcesUnique = append(sourcesUnique, source)
		}
	}

	return sourcesUnique, whitelist
}

func makeCSPHeader(parent, siblings, header, cspWhitelist string, sources []CSPSource, isSecure bool) string {
	headers := make([]string, len(sources))
	for i, src := range sources {
		switch src {
		case CSPSrcSelf:
			headers[i] = "'self'"
		case CSPSrcData:
			headers[i] = "data:"
		case CSPSrcBlob:
			headers[i] = "blob:"
		case CSPSrcParent:
			if isSecure {
				headers[i] = "https://" + parent
			} else {
				headers[i] = "http://" + parent
			}
		case CSPSrcWS:
			if isSecure {
				headers[i] = "wss://" + parent
			} else {
				headers[i] = "ws://" + parent
			}
		case CSPSrcSiblings:
			if isSecure {
				headers[i] = "https://" + siblings
			} else {
				headers[i] = "http://" + siblings
			}
		case CSPSrcAny:
			headers[i] = "*"
		case CSPUnsafeInline:
			headers[i] = "'unsafe-inline'"
		case CSPWhitelist:
			headers[i] = cspWhitelist
		}
	}
	return header + " " + strings.Join(headers, " ") + ";"
}
