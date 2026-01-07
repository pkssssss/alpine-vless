package singbox

import (
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
)

func wrapHTTPDoError(err error) error {
	if err == nil {
		return nil
	}

	if isTLSCertError(err) {
		return fmt.Errorf("HTTPS 请求失败，可能缺少 CA 证书（Alpine 常见修复：apk add --no-cache ca-certificates && update-ca-certificates）：%w", err)
	}
	return err
}

func isTLSCertError(err error) bool {
	var ua x509.UnknownAuthorityError
	if errors.As(err, &ua) {
		return true
	}
	var ci x509.CertificateInvalidError
	if errors.As(err, &ci) {
		return true
	}
	return strings.Contains(err.Error(), "x509:")
}

