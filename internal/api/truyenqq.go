package api

const (
	truyenqqPrimaryURL  = "https://truyenqqko.com"
	truyenqqFallbackURL = "https://metruyenqq.net"
)

func NewTruyenQQProvider() *truyenHTMLProvider {
	return newTruyenHTMLProvider("TruyenQQ", truyenqqPrimaryURL, truyenqqFallbackURL)
}
