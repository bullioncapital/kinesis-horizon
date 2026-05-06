package actions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stellar/go/network"
	"github.com/stellar/go/services/horizon/internal/corestate"
	hProblem "github.com/stellar/go/services/horizon/internal/render/problem"
	"github.com/stellar/go/services/horizon/internal/txsub"
	"github.com/stellar/go/support/render/problem"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestStellarCoreMalformedTx(t *testing.T) {
	handler := SubmitTransactionHandler{}

	r := httptest.NewRequest("POST", "https://horizon.stellar.org/transactions", nil)
	w := httptest.NewRecorder()
	_, err := handler.GetResource(w, r)
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, err.(*problem.P).Status)
	assert.Equal(t, "Transaction Malformed", err.(*problem.P).Title)
}

type coreStateGetterMock struct {
	mock.Mock
}

func (m *coreStateGetterMock) GetCoreState() corestate.State {
	a := m.Called()
	return a.Get(0).(corestate.State)
}

type networkSubmitterMock struct {
	mock.Mock
}

func (m *networkSubmitterMock) Submit(ctx context.Context, rawTx string, envelope xdr.TransactionEnvelope, hash string) <-chan txsub.Result {
	a := m.Called()
	return a.Get(0).(chan txsub.Result)
}

func TestStellarCoreNotSynced(t *testing.T) {
	mock := &coreStateGetterMock{}
	mock.On("GetCoreState").Return(corestate.State{
		Synced: false,
	})

	handler := SubmitTransactionHandler{
		NetworkPassphrase: network.PublicNetworkPassphrase,
		CoreStateGetter:   mock,
	}

	form := url.Values{}
	form.Set("tx", "AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAK6jei3jmoI8TGlD/egc37PXtHKKzWV8wViZBaCu5L5MAAAAADuaygAAAAAAAAAAAVb8BfcAAABAg86r5EAUKDQCYnz0Vw6C4b7cnE95RTwkOdYJHbBR2gTVsNOUv1YVtF4JK9AgTxODWhVdipnLN2cC5om+E0azCw==")

	request, err := http.NewRequest(
		"POST",
		"https://horizon.stellar.org/transactions",
		strings.NewReader(form.Encode()),
	)
	require.NoError(t, err)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	_, err = handler.GetResource(w, request)
	assert.Error(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, err.(problem.P).Status)
	assert.Equal(t, "stale_history", err.(problem.P).Type)
	assert.Equal(t, "Historical DB Is Too Stale", err.(problem.P).Title)
}

func TestTimeoutSubmission(t *testing.T) {
	mockSubmitChannel := make(chan txsub.Result)

	mock := &coreStateGetterMock{}
	mock.On("GetCoreState").Return(corestate.State{
		Synced: true,
	})

	mockSubmitter := &networkSubmitterMock{}
	mockSubmitter.On("Submit").Return(mockSubmitChannel)

	handler := SubmitTransactionHandler{
		Submitter:         mockSubmitter,
		NetworkPassphrase: network.PublicNetworkPassphrase,
		CoreStateGetter:   mock,
	}

	form := url.Values{}
	form.Set("tx", "AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAK6jei3jmoI8TGlD/egc37PXtHKKzWV8wViZBaCu5L5MAAAAADuaygAAAAAAAAAAAVb8BfcAAABAg86r5EAUKDQCYnz0Vw6C4b7cnE95RTwkOdYJHbBR2gTVsNOUv1YVtF4JK9AgTxODWhVdipnLN2cC5om+E0azCw==")

	request, err := http.NewRequest(
		"POST",
		"https://horizon.stellar.org/transactions",
		strings.NewReader(form.Encode()),
	)

	require.NoError(t, err)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	ctx, cancel := context.WithTimeout(request.Context(), time.Duration(0))
	defer cancel()
	request = request.WithContext(ctx)

	w := httptest.NewRecorder()
	_, err = handler.GetResource(w, request)
	assert.Error(t, err)
	assert.Equal(t, hProblem.Timeout, err)
}

func TestClientDisconnectSubmission(t *testing.T) {
	mockSubmitChannel := make(chan txsub.Result)

	mock := &coreStateGetterMock{}
	mock.On("GetCoreState").Return(corestate.State{
		Synced: true,
	})

	mockSubmitter := &networkSubmitterMock{}
	mockSubmitter.On("Submit").Return(mockSubmitChannel)

	handler := SubmitTransactionHandler{
		Submitter:         mockSubmitter,
		NetworkPassphrase: network.PublicNetworkPassphrase,
		CoreStateGetter:   mock,
	}

	form := url.Values{}
	form.Set("tx", "AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAK6jei3jmoI8TGlD/egc37PXtHKKzWV8wViZBaCu5L5MAAAAADuaygAAAAAAAAAAAVb8BfcAAABAg86r5EAUKDQCYnz0Vw6C4b7cnE95RTwkOdYJHbBR2gTVsNOUv1YVtF4JK9AgTxODWhVdipnLN2cC5om+E0azCw==")

	request, err := http.NewRequest(
		"POST",
		"https://horizon.stellar.org/transactions",
		strings.NewReader(form.Encode()),
	)

	require.NoError(t, err)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	ctx, cancel := context.WithCancel(request.Context())
	cancel()
	request = request.WithContext(ctx)

	w := httptest.NewRecorder()
	_, err = handler.GetResource(w, request)
	assert.Equal(t, hProblem.ClientDisconnected, err)
}
