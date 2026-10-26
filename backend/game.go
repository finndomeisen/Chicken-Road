package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	totalFields           = 10
	initialBalanceCents   = int64(10000)
	minimumBetCents       = int64(1)
	maximumBetCents       = int64(10000000)
	defaultFrontendOrigin = "http://localhost:5173"
)

var playerIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type app struct {
	db             *sql.DB
	frontendOrigin string
	randomIntn     func(int) (int, error)
	mu             sync.Mutex
	closeOnce      sync.Once
	closeErr       error
	handler        http.Handler
}

type accountResponse struct {
	PlayerID    string             `json:"playerId"`
	Balance     float64            `json:"balance"`
	ActiveRound *gameRoundResponse `json:"activeRound"`
}

type gameRoundResponse struct {
	GameID                 string  `json:"gameId"`
	BetAmount              float64 `json:"betAmount"`
	CurrentField           int     `json:"currentField"`
	TotalFields            int     `json:"totalFields"`
	NextFieldDangerPercent *int    `json:"nextFieldDangerPercent"`
	ProfitAmount           float64 `json:"profitAmount"`
	CashoutAmount          float64 `json:"cashoutAmount"`
	Balance                float64 `json:"balance"`
	Status                 string  `json:"status"`
	LastFieldResult        string  `json:"lastFieldResult"`
	Message                string  `json:"message"`
}

type storedRound struct {
	gameID          string
	playerID        string
	betCents        int64
	currentField    int
	status          string
	lastFieldResult string
}

type apiError struct {
	Error string `json:"error"`
}

func newApp(databasePath, frontendOrigin string) (*app, error) {
	if frontendOrigin == "" {
		frontendOrigin = defaultFrontendOrigin
	}
	if databasePath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(databasePath), 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)

	for _, statement := range []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = WAL`,
		`CREATE TABLE IF NOT EXISTS players (
			player_id TEXT PRIMARY KEY,
			balance_cents INTEGER NOT NULL CHECK (balance_cents >= 0),
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS game_rounds (
			game_id TEXT PRIMARY KEY,
			player_id TEXT NOT NULL REFERENCES players(player_id),
			bet_cents INTEGER NOT NULL CHECK (bet_cents > 0),
			current_field INTEGER NOT NULL DEFAULT 0 CHECK (current_field BETWEEN 0 AND 10),
			status TEXT NOT NULL CHECK (status IN ('ACTIVE', 'LOST', 'CASHED_OUT', 'COMPLETED')),
			last_field_result TEXT NOT NULL CHECK (last_field_result IN ('NONE', 'SAFE', 'HIT')),
			created_at TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS one_active_round_per_player
			ON game_rounds(player_id) WHERE status = 'ACTIVE'`,
	} {
		if _, err := db.Exec(statement); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("initialize database: %w", err)
		}
	}

	application := &app{
		db:             db,
		frontendOrigin: frontendOrigin,
		randomIntn: func(max int) (int, error) {
			value, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
			if err != nil {
				return 0, err
			}
			return int(value.Int64()), nil
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/game/account", application.getAccount)
	mux.HandleFunc("POST /api/game/start", application.startRound)
	mux.HandleFunc("POST /api/game/{gameId}/step", application.takeStep)
	mux.HandleFunc("POST /api/game/{gameId}/cashout", application.cashOut)
	application.handler = mux
	return application, nil
}

func (a *app) close() error {
	a.closeOnce.Do(func() {
		a.closeErr = a.db.Close()
	})
	return a.closeErr
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin != "" && origin == a.frontendOrigin {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Player-Id")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	a.handler.ServeHTTP(w, r)
}

func (a *app) getAccount(w http.ResponseWriter, r *http.Request) {
	playerID, ok := readPlayerID(w, r)
	if !ok {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Das Konto konnte nicht geladen werden.")
		return
	}
	defer tx.Rollback()

	if err := ensurePlayer(tx, playerID); err != nil {
		writeError(w, http.StatusInternalServerError, "Das Konto konnte nicht geladen werden.")
		return
	}

	var balance int64
	if err := tx.QueryRowContext(r.Context(),
		`SELECT balance_cents FROM players WHERE player_id = ?`, playerID).Scan(&balance); err != nil {
		writeError(w, http.StatusInternalServerError, "Das Konto konnte nicht geladen werden.")
		return
	}

	response := accountResponse{
		PlayerID: playerID,
		Balance:  amount(balance),
	}
	round, err := findActiveRound(tx, playerID)
	switch {
	case err == nil:
		roundResponse := responseFor(round, balance, "Deine Runde läuft weiter.")
		response.ActiveRound = &roundResponse
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		writeError(w, http.StatusInternalServerError, "Die laufende Runde konnte nicht geladen werden.")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Das Konto konnte nicht geladen werden.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *app) startRound(w http.ResponseWriter, r *http.Request) {
	playerID, ok := readPlayerID(w, r)
	if !ok {
		return
	}

	var request struct {
		BetAmount json.RawMessage `json:"betAmount"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage. Bitte gib einen Einsatz als JSON-Zahl an.")
		return
	}
	betJSON := strings.TrimSpace(string(request.BetAmount))
	if betJSON == "" || (betJSON[0] != '-' && (betJSON[0] < '0' || betJSON[0] > '9')) {
		writeError(w, http.StatusBadRequest, "Bitte gib den Einsatz als JSON-Zahl an, nicht als Text.")
		return
	}
	betCents, err := parseBet(betJSON)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Die Runde konnte nicht gestartet werden.")
		return
	}
	defer tx.Rollback()

	if err := ensurePlayer(tx, playerID); err != nil {
		writeError(w, http.StatusInternalServerError, "Das Konto konnte nicht geladen werden.")
		return
	}
	var activeCount int
	if err := tx.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM game_rounds WHERE player_id = ? AND status = 'ACTIVE'`, playerID).Scan(&activeCount); err != nil {
		writeError(w, http.StatusInternalServerError, "Der Rundenstatus konnte nicht geprüft werden.")
		return
	}
	if activeCount > 0 {
		writeError(w, http.StatusConflict, "Du hast bereits eine laufende Runde.")
		return
	}

	var balance int64
	if err := tx.QueryRowContext(r.Context(),
		`SELECT balance_cents FROM players WHERE player_id = ?`, playerID).Scan(&balance); err != nil {
		writeError(w, http.StatusInternalServerError, "Das Konto konnte nicht geladen werden.")
		return
	}
	if balance < betCents {
		writeError(w, http.StatusBadRequest, "Dein Guthaben reicht für diesen Einsatz nicht aus.")
		return
	}
	balance -= betCents
	if _, err := tx.ExecContext(r.Context(),
		`UPDATE players SET balance_cents = ? WHERE player_id = ?`, balance, playerID); err != nil {
		writeError(w, http.StatusInternalServerError, "Der Einsatz konnte nicht abgezogen werden.")
		return
	}

	gameID, err := newUUID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Die Runde konnte nicht gestartet werden.")
		return
	}
	round := storedRound{
		gameID:          gameID,
		playerID:        playerID,
		betCents:        betCents,
		status:          "ACTIVE",
		lastFieldResult: "NONE",
	}
	if _, err := tx.ExecContext(r.Context(),
		`INSERT INTO game_rounds (game_id, player_id, bet_cents, current_field, status, last_field_result, created_at)
		 VALUES (?, ?, ?, 0, 'ACTIVE', 'NONE', ?)`,
		round.gameID, round.playerID, round.betCents, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		writeError(w, http.StatusInternalServerError, "Die Runde konnte nicht gespeichert werden.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Die Runde konnte nicht gespeichert werden.")
		return
	}

	writeJSON(w, http.StatusOK, responseFor(round, balance, "Runde gestartet. Wage deinen ersten Schritt oder zahle jederzeit aus."))
}

func (a *app) takeStep(w http.ResponseWriter, r *http.Request) {
	playerID, ok := readPlayerID(w, r)
	if !ok {
		return
	}
	gameID := r.PathValue("gameId")
	if !isUUID(gameID) {
		writeError(w, http.StatusNotFound, "Spielrunde nicht gefunden.")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Der Spielzug konnte nicht gespeichert werden.")
		return
	}
	defer tx.Rollback()

	round, err := loadRound(tx, playerID, gameID)
	if err != nil {
		writeRoundLookupError(w, err)
		return
	}
	if round.status != "ACTIVE" {
		writeError(w, http.StatusBadRequest, "Diese Spielrunde ist bereits beendet.")
		return
	}

	dangerPercent := dangerPercentForField(round.currentField)
	draw, err := a.randomIntn(1000000)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Der Spielzug konnte nicht ausgelost werden.")
		return
	}

	balance, err := readBalance(tx, playerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Das Konto konnte nicht geladen werden.")
		return
	}
	message := "Sicher angekommen! Du kannst weitergehen oder jetzt auszahlen."
	if draw < dangerPercent*10000 {
		round.status = "LOST"
		round.lastFieldResult = "HIT"
		message = fmt.Sprintf("Das Huhn wurde auf Feld %d überfahren. Der Einsatz ist verloren.", round.currentField+1)
	} else {
		round.currentField++
		round.lastFieldResult = "SAFE"
		if round.currentField == totalFields {
			round.status = "COMPLETED"
			balance += cashoutCents(round.betCents, round.currentField)
			if err := updateBalance(tx, playerID, balance); err != nil {
				writeError(w, http.StatusInternalServerError, "Die Auszahlung konnte nicht gutgeschrieben werden.")
				return
			}
			message = "Alle 10 Felder geschafft! Die Auszahlung wurde deinem Guthaben gutgeschrieben."
		}
	}

	if err := updateRound(tx, round); err != nil {
		writeError(w, http.StatusInternalServerError, "Der Spielzug konnte nicht gespeichert werden.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Der Spielzug konnte nicht gespeichert werden.")
		return
	}
	writeJSON(w, http.StatusOK, responseFor(round, balance, message))
}

func (a *app) cashOut(w http.ResponseWriter, r *http.Request) {
	playerID, ok := readPlayerID(w, r)
	if !ok {
		return
	}
	gameID := r.PathValue("gameId")
	if !isUUID(gameID) {
		writeError(w, http.StatusNotFound, "Spielrunde nicht gefunden.")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Die Auszahlung konnte nicht gespeichert werden.")
		return
	}
	defer tx.Rollback()

	round, err := loadRound(tx, playerID, gameID)
	if err != nil {
		writeRoundLookupError(w, err)
		return
	}
	if round.status != "ACTIVE" {
		writeError(w, http.StatusBadRequest, "Diese Spielrunde ist bereits beendet.")
		return
	}

	balance, err := readBalance(tx, playerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Das Konto konnte nicht geladen werden.")
		return
	}
	round.status = "CASHED_OUT"
	balance += cashoutCents(round.betCents, round.currentField)
	if err := updateBalance(tx, playerID, balance); err != nil {
		writeError(w, http.StatusInternalServerError, "Die Auszahlung konnte nicht gutgeschrieben werden.")
		return
	}
	if err := updateRound(tx, round); err != nil {
		writeError(w, http.StatusInternalServerError, "Die Runde konnte nicht beendet werden.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Die Auszahlung konnte nicht gespeichert werden.")
		return
	}

	writeJSON(w, http.StatusOK, responseFor(round, balance, "Auszahlung deinem Guthaben gutgeschrieben."))
}

func ensurePlayer(tx *sql.Tx, playerID string) error {
	_, err := tx.Exec(
		`INSERT INTO players (player_id, balance_cents, created_at) VALUES (?, ?, ?)
		 ON CONFLICT(player_id) DO NOTHING`,
		playerID, initialBalanceCents, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func findActiveRound(tx *sql.Tx, playerID string) (storedRound, error) {
	var round storedRound
	err := tx.QueryRow(
		`SELECT game_id, player_id, bet_cents, current_field, status, last_field_result
		 FROM game_rounds WHERE player_id = ? AND status = 'ACTIVE'`,
		playerID).Scan(
		&round.gameID, &round.playerID, &round.betCents, &round.currentField, &round.status, &round.lastFieldResult)
	return round, err
}

func loadRound(tx *sql.Tx, playerID, gameID string) (storedRound, error) {
	var round storedRound
	err := tx.QueryRow(
		`SELECT game_id, player_id, bet_cents, current_field, status, last_field_result
		 FROM game_rounds WHERE player_id = ? AND game_id = ?`,
		playerID, gameID).Scan(
		&round.gameID, &round.playerID, &round.betCents, &round.currentField, &round.status, &round.lastFieldResult)
	return round, err
}

func readBalance(tx *sql.Tx, playerID string) (int64, error) {
	var balance int64
	err := tx.QueryRow(`SELECT balance_cents FROM players WHERE player_id = ?`, playerID).Scan(&balance)
	return balance, err
}

func updateBalance(tx *sql.Tx, playerID string, balance int64) error {
	_, err := tx.Exec(`UPDATE players SET balance_cents = ? WHERE player_id = ?`, balance, playerID)
	return err
}

func updateRound(tx *sql.Tx, round storedRound) error {
	_, err := tx.Exec(
		`UPDATE game_rounds SET current_field = ?, status = ?, last_field_result = ? WHERE game_id = ? AND player_id = ?`,
		round.currentField, round.status, round.lastFieldResult, round.gameID, round.playerID)
	return err
}

func responseFor(round storedRound, balance int64, message string) gameRoundResponse {
	profit := int64(0)
	if round.status != "LOST" && round.currentField > 0 {
		profit = profitCents(round.betCents, round.currentField)
	}

	cashout := int64(0)
	if round.status != "LOST" {
		cashout = round.betCents + profit
	}

	var nextDanger *int
	if round.status == "ACTIVE" && round.currentField < totalFields {
		danger := dangerPercentForField(round.currentField)
		nextDanger = &danger
	}

	return gameRoundResponse{
		GameID:                 round.gameID,
		BetAmount:              amount(round.betCents),
		CurrentField:           round.currentField,
		TotalFields:            totalFields,
		NextFieldDangerPercent: nextDanger,
		ProfitAmount:           amount(profit),
		CashoutAmount:          amount(cashout),
		Balance:                amount(balance),
		Status:                 round.status,
		LastFieldResult:        round.lastFieldResult,
		Message:                message,
	}
}

func dangerPercentForField(zeroBasedField int) int {
	return 10 + 5*zeroBasedField
}

func profitCents(betCents int64, safeFields int) int64 {
	if safeFields <= 0 {
		return 0
	}
	profit := (betCents*10 + 50) / 100
	return profit << (safeFields - 1)
}

func cashoutCents(betCents int64, safeFields int) int64 {
	return betCents + profitCents(betCents, safeFields)
}

func parseBet(value string) (int64, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("Der Einsatz muss eine Zahl mit maximal zwei Dezimalstellen sein.")
	}
	if strings.HasPrefix(parts[0], "-") || strings.HasPrefix(parts[0], "+") {
		return 0, errors.New("Der Einsatz muss positiv sein.")
	}
	for _, part := range parts {
		if part == "" && len(parts) > 1 {
			return 0, errors.New("Der Einsatz muss eine Zahl mit maximal zwei Dezimalstellen sein.")
		}
		if _, err := strconv.ParseUint(part, 10, 64); err != nil {
			return 0, errors.New("Der Einsatz muss eine Zahl mit maximal zwei Dezimalstellen sein.")
		}
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) > 2 {
			return 0, errors.New("Der Einsatz darf maximal zwei Dezimalstellen haben.")
		}
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > maximumBetCents/100 {
		return 0, errors.New("Der Einsatz darf höchstens CHF 100'000 betragen.")
	}
	fractionalCents := int64(0)
	if fraction != "" {
		fractionalCents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, errors.New("Der Einsatz muss eine gültige Zahl sein.")
		}
	}
	cents := whole*100 + fractionalCents
	if cents < minimumBetCents {
		return 0, errors.New("Der Einsatz muss mindestens CHF 0.01 betragen.")
	}
	if cents > maximumBetCents {
		return 0, errors.New("Der Einsatz darf höchstens CHF 100'000 betragen.")
	}
	return cents, nil
}

func amount(cents int64) float64 {
	return float64(cents) / 100
}

func decodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

func readPlayerID(w http.ResponseWriter, r *http.Request) (string, bool) {
	playerID := r.Header.Get("X-Player-Id")
	if !isUUID(playerID) {
		writeError(w, http.StatusBadRequest, "Die Spieler-ID fehlt oder ist ungültig.")
		return "", false
	}
	return strings.ToLower(playerID), true
}

func isUUID(value string) bool {
	return playerIDPattern.MatchString(value)
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate round ID: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	hex := fmt.Sprintf("%x", value)
	return hex[:8] + "-" + hex[8:12] + "-" + hex[12:16] + "-" + hex[16:20] + "-" + hex[20:], nil
}

func writeRoundLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Spielrunde nicht gefunden.")
		return
	}
	writeError(w, http.StatusInternalServerError, "Die Spielrunde konnte nicht geladen werden.")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Printf("write JSON response: %v\n", err)
	}
}
