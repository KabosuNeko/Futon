package api

const truyenggBaseURL = "https://truyenggvn.com"

func NewTruyenGGProvider() *truyenHTMLProvider {
	return newTruyenHTMLProvider("TruyenGG", truyenggBaseURL)
}
