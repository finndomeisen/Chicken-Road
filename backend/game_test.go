package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
)

type mockedRandom struct {
	mock.Mock
}

func (m *mockedRandom) Intn(max int) (int, error) {
	args := m.Called(max)
	return args.Int(0), args.Error(1)
}

func newTestApp(t *testing.T, databasePath string) *app {
	t.Helper()
	if databasePath == "" {
		databasePath = filepath.Join(t.TempDir(), "chicken-road.sqlite")
	}
	application, err := newApp(databasePath, defaultFrontendOrigin)
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	t.Cleanup(func() {
		if err := application.close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	application.randomIntn = func(int) (int, error) {
		return 999999, nil
	}
	return application
}

func expectRandom(t *testing.T, application *app, result, calls int) *mockedRandom {
	t.Helper()
	random := &mockedRandom{}
	random.Test(t)
	random.On("Intn", 1000000).Return(result, nil).Times(calls)
	application.randomIntn = random.Intn
	t.Cleanup(func() {
		random.AssertExpectations(t)
	})
	return random
}

func performRequest(t *testing.T, handler http.Handler, method, path, playerID string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if playerID != "" {
		request.Header.Set("X-Player-Id", playerID)
	}
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeResponse[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
	return value
}

func TestAccountRoundAndCashoutPersistAcrossRestart(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "data", "chicken-road.sqlite")
	playerID := "414b9a56-d4e1-4cd0-9e9e-004dcf9e5621"
	application := newTestApp(t, databasePath)

	accountHTTPResponse := performRequest(t, application, http.MethodGet, "/api/game/account", playerID, nil)
	if accountHTTPResponse.Code != http.StatusOK {
		t.Fatalf("get account: status %d: %s", accountHTTPResponse.Code, accountHTTPResponse.Body.String())
	}
	account := decodeResponse[accountResponse](t, accountHTTPResponse)
	if account.Balance != 100 {
		t.Fatalf("initial balance = %v, want 100", account.Balance)
	}

	startResponse := performRequest(t, application, http.MethodPost, "/api/game/start", playerID, []byte(`{"betAmount":25}`))
	if startResponse.Code != http.StatusOK {
		t.Fatalf("start round: status %d: %s", startResponse.Code, startResponse.Body.String())
	}
	started := decodeResponse[gameRoundResponse](t, startResponse)
	if started.Balance != 75 || started.NextFieldDangerPercent == nil || *started.NextFieldDangerPercent != 10 {
		t.Fatalf("unexpected new round: %+v", started)
	}

	stepResponse := performRequest(t, application, http.MethodPost, "/api/game/"+started.GameID+"/step", playerID, nil)
	if stepResponse.Code != http.StatusOK {
		t.Fatalf("take step: status %d: %s", stepResponse.Code, stepResponse.Body.String())
	}
	stepped := decodeResponse[gameRoundResponse](t, stepResponse)
	if stepped.CurrentField != 1 || stepped.ProfitAmount != 2.5 || stepped.Balance != 75 {
		t.Fatalf("unexpected step response: %+v", stepped)
	}

	if err := application.close(); err != nil {
		t.Fatalf("close app before restart: %v", err)
	}
	restarted := newTestApp(t, databasePath)
	restartedAccountHTTPResponse := performRequest(t, restarted, http.MethodGet, "/api/game/account", playerID, nil)
	if restartedAccountHTTPResponse.Code != http.StatusOK {
		t.Fatalf("reload account: status %d: %s", restartedAccountHTTPResponse.Code, restartedAccountHTTPResponse.Body.String())
	}
	reloaded := decodeResponse[accountResponse](t, restartedAccountHTTPResponse)
	if reloaded.Balance != 75 || reloaded.ActiveRound == nil {
		t.Fatalf("account or active round was not restored: %+v", reloaded)
	}
	if reloaded.ActiveRound.GameID != started.GameID || reloaded.ActiveRound.CurrentField != 1 {
		t.Fatalf("round progress was not restored: %+v", reloaded.ActiveRound)
	}

	cashoutResponse := performRequest(t, restarted, http.MethodPost, "/api/game/"+started.GameID+"/cashout", playerID, nil)
	if cashoutResponse.Code != http.StatusOK {
		t.Fatalf("cash out: status %d: %s", cashoutResponse.Code, cashoutResponse.Body.String())
	}
	cashedOut := decodeResponse[gameRoundResponse](t, cashoutResponse)
	if cashedOut.Balance != 102.5 || cashedOut.Status != "CASHED_OUT" {
		t.Fatalf("unexpected cashout result: %+v", cashedOut)
	}
}

func TestHitConsumesStakeAndDoesNotCreditUnsecuredProfit(t *testing.T) {
	application := newTestApp(t, "")
	playerID := "414b9a56-d4e1-4cd0-9e9e-004dcf9e5622"
	expectRandom(t, application, 0, 1)

	performRequest(t, application, http.MethodGet, "/api/game/account", playerID, nil)
	startResponse := performRequest(t, application, http.MethodPost, "/api/game/start", playerID, []byte(`{"betAmount":10}`))
	started := decodeResponse[gameRoundResponse](t, startResponse)
	hitResponse := performRequest(t, application, http.MethodPost, "/api/game/"+started.GameID+"/step", playerID, nil)
	hit := decodeResponse[gameRoundResponse](t, hitResponse)

	if hit.Status != "LOST" || hit.LastFieldResult != "HIT" || hit.CashoutAmount != 0 || hit.Balance != 90 {
		t.Fatalf("unexpected hit result: %+v", hit)
	}
}

func TestCashoutBeforeFirstStepRefundsStakeWithoutProfit(t *testing.T) {
	application := newTestApp(t, "")
	playerID := "414b9a56-d4e1-4cd0-9e9e-004dcf9e5626"

	performRequest(t, application, http.MethodGet, "/api/game/account", playerID, nil)
	startResponse := performRequest(t, application, http.MethodPost, "/api/game/start", playerID, []byte(`{"betAmount":10}`))
	started := decodeResponse[gameRoundResponse](t, startResponse)
	cashoutResponse := performRequest(t, application, http.MethodPost, "/api/game/"+started.GameID+"/cashout", playerID, nil)
	cashedOut := decodeResponse[gameRoundResponse](t, cashoutResponse)

	if cashedOut.ProfitAmount != 0 || cashedOut.CashoutAmount != 10 || cashedOut.Balance != 100 {
		t.Fatalf("unexpected immediate cashout: %+v", cashedOut)
	}
}

func TestRejectsInvalidAndUnaffordableBets(t *testing.T) {
	application := newTestApp(t, "")
	playerID := "414b9a56-d4e1-4cd0-9e9e-004dcf9e5623"

	for _, body := range []string{
		`{"betAmount":0}`,
		`{"betAmount":-1}`,
		`{"betAmount":0.001}`,
		`{"betAmount":100000.01}`,
		`{"betAmount":"10"}`,
		`{"betAmount":}`,
	} {
		response := performRequest(t, application, http.MethodPost, "/api/game/start", playerID, []byte(body))
		if response.Code < http.StatusBadRequest || response.Code >= http.StatusInternalServerError {
			t.Errorf("bet %s returned status %d, want 4xx", body, response.Code)
		}
	}

	response := performRequest(t, application, http.MethodPost, "/api/game/start", playerID, []byte(`{"betAmount":100.01}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unaffordable bet returned status %d, want 400", response.Code)
	}
}

func TestPreventsMultipleActiveRoundsAndForeignRoundAccess(t *testing.T) {
	application := newTestApp(t, "")
	playerID := "414b9a56-d4e1-4cd0-9e9e-004dcf9e5624"
	otherPlayerID := "414b9a56-d4e1-4cd0-9e9e-004dcf9e5625"

	performRequest(t, application, http.MethodGet, "/api/game/account", playerID, nil)
	startResponse := performRequest(t, application, http.MethodPost, "/api/game/start", playerID, []byte(`{"betAmount":10}`))
	started := decodeResponse[gameRoundResponse](t, startResponse)

	secondStart := performRequest(t, application, http.MethodPost, "/api/game/start", playerID, []byte(`{"betAmount":10}`))
	if secondStart.Code != http.StatusConflict {
		t.Fatalf("second active round returned status %d, want 409", secondStart.Code)
	}
	foreignStep := performRequest(t, application, http.MethodPost, "/api/game/"+started.GameID+"/step", otherPlayerID, nil)
	if foreignStep.Code != http.StatusNotFound {
		t.Fatalf("foreign round returned status %d, want 404", foreignStep.Code)
	}
}

func TestValidatesPlayerIDAndHandlesCorsPreflight(t *testing.T) {
	application := newTestApp(t, "")
	missingID := performRequest(t, application, http.MethodGet, "/api/game/account", "", nil)
	if missingID.Code != http.StatusBadRequest {
		t.Fatalf("missing player ID returned status %d, want 400", missingID.Code)
	}

	request := httptest.NewRequest(http.MethodOptions, "/api/game/account", nil)
	request.Header.Set("Origin", defaultFrontendOrigin)
	response := httptest.NewRecorder()
	application.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("CORS preflight returned status %d, want 204", response.Code)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != defaultFrontendOrigin {
		t.Fatalf("unexpected allowed origin %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestParseBet(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{input: "10", want: 1000},
		{input: "10.5", want: 1050},
		{input: "0.01", want: 1},
		{input: "10.123", wantErr: true},
		{input: "1e2", wantErr: true},
		{input: "-1", wantErr: true},
		{input: "100000.01", wantErr: true},
	}

	for _, test := range tests {
		got, err := parseBet(test.input)
		if (err != nil) != test.wantErr {
			t.Errorf("parseBet(%q) error = %v, wantErr %v", test.input, err, test.wantErr)
		}
		if err == nil && got != test.want {
			t.Errorf("parseBet(%q) = %d, want %d", test.input, got, test.want)
		}
	}
}

func TestRiskAndPayoutProgression(t *testing.T) {
	wantDanger := []int{10, 15, 20, 25, 30, 35, 40, 45, 50, 55}
	wantProfitCents := []int64{0, 100, 200, 400, 800, 1600, 3200, 6400, 12800, 25600, 51200}
	const betCents = int64(1000)

	for safeFields, want := range wantProfitCents {
		if got := profitCents(betCents, safeFields); got != want {
			t.Errorf("profitCents(%d, %d) = %d, want %d", betCents, safeFields, got, want)
		}
		if got := cashoutCents(betCents, safeFields); got != betCents+want {
			t.Errorf("cashoutCents(%d, %d) = %d, want %d", betCents, safeFields, got, betCents+want)
		}
	}

	for field, want := range wantDanger {
		if got := dangerPercentForField(field); got != want {
			t.Errorf("dangerPercentForField(%d) = %d, want %d", field, got, want)
		}
	}
}

func TestCompletingAllFieldsPaysOutAutomatically(t *testing.T) {
	application := newTestApp(t, "")
	playerID := "414b9a56-d4e1-4cd0-9e9e-004dcf9e5627"
	expectRandom(t, application, 999999, totalFields)

	accountResponse := performRequest(t, application, http.MethodGet, "/api/game/account", playerID, nil)
	if accountResponse.Code != http.StatusOK {
		t.Fatalf("get account: status %d: %s", accountResponse.Code, accountResponse.Body.String())
	}
	startResponse := performRequest(t, application, http.MethodPost, "/api/game/start", playerID, []byte(`{"betAmount":10}`))
	if startResponse.Code != http.StatusOK {
		t.Fatalf("start round: status %d: %s", startResponse.Code, startResponse.Body.String())
	}
	started := decodeResponse[gameRoundResponse](t, startResponse)

	var result gameRoundResponse
	for field := 0; field < totalFields; field++ {
		response := performRequest(t, application, http.MethodPost, "/api/game/"+started.GameID+"/step", playerID, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("step %d: status %d: %s", field+1, response.Code, response.Body.String())
		}
		result = decodeResponse[gameRoundResponse](t, response)
	}

	if result.Status != "COMPLETED" || result.CurrentField != totalFields || result.NextFieldDangerPercent != nil {
		t.Fatalf("unexpected completed round: %+v", result)
	}
	if result.ProfitAmount != 512 || result.CashoutAmount != 522 || result.Balance != 612 {
		t.Fatalf("unexpected final payout: %+v", result)
	}
}
