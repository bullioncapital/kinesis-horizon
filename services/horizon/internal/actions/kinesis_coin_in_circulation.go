package actions

import (
	"net/http"

	"github.com/stellar/go/protocols/horizon"
)

type KinesisCoinInCirculationQuery struct {
	From string `schema:"from"`
}

// URITemplate returns a rfc6570 URI template the query struct
func (q KinesisCoinInCirculationQuery) URITemplate() string {
	return getURITemplate(&q, "coin_in_circulation", true)
}

type KinesisCoinInCirculationHandler struct{}

func (handler KinesisCoinInCirculationHandler) GetResource(w HeaderWriter, r *http.Request) (interface{}, error) {
	cic := horizon.KinesisCoinInCirculation{}
	cic.Echo = "Hello World!"
	cic.State = "Partial"
	return cic, nil
}
