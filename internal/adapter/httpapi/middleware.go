package httpapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

const (
	headerUser        = "X-API-User"
	headerPass        = "X-API-Password"
	headerWorkstation = "X-Irbis-Workstation"

	cookieUser        = "irbis_user"
	cookiePass        = "irbis_password"
	cookieWorkstation = "irbis_workstation"

	cookieMaxAge = 7 * 24 * time.Hour
)

func credentials(c echo.Context) (user, pass string) {
	user = c.Request().Header.Get(headerUser)
	pass = c.Request().Header.Get(headerPass)
	if user == "" {
		if ck, err := c.Cookie(cookieUser); err == nil {
			user = ck.Value
		}
	}
	if pass == "" {
		if ck, err := c.Cookie(cookiePass); err == nil {
			pass = ck.Value
		}
	}
	return user, pass
}

func workstation(c echo.Context) string {
	if v := c.Request().Header.Get(headerWorkstation); v != "" {
		return v
	}
	if ck, err := c.Cookie(cookieWorkstation); err == nil {
		return ck.Value
	}
	return ""
}

func setAuthCookies(c echo.Context, user, pass, arm string) {
	secure := c.Scheme() == "https"
	setCookie(c, cookieUser, user, secure)
	setCookie(c, cookiePass, pass, secure)
	if arm != "" {
		setCookie(c, cookieWorkstation, arm, secure)
	}
}

func clearAuthCookies(c echo.Context) {
	secure := c.Scheme() == "https"
	expireCookie(c, cookieUser, secure)
	expireCookie(c, cookiePass, secure)
	expireCookie(c, cookieWorkstation, secure)
}

func setCookie(c echo.Context, name, value string, secure bool) {
	c.SetCookie(&http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(cookieMaxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

func expireCookie(c echo.Context, name string, secure bool) {
	c.SetCookie(&http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}
