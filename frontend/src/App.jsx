import React, { useEffect, useState } from 'react';
import axios from 'axios';
import './App.css';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/game';
const formatChf = (amount) => new Intl.NumberFormat('de-CH', {
  style: 'currency',
  currency: 'CHF'
}).format(amount);

function App() {
  const [bet, setBet] = useState('10.00');
  const [playerId, setPlayerId] = useState(null);
  const [account, setAccount] = useState(null);
  const [round, setRound] = useState(null);
  const [isBusy, setIsBusy] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;

    const loadAccount = async () => {
      try {
        let storedPlayerId = localStorage.getItem('chicken-road-player-id');
        if (!storedPlayerId || !/^[0-9a-f-]{36}$/i.test(storedPlayerId)) {
          storedPlayerId = crypto.randomUUID();
          localStorage.setItem('chicken-road-player-id', storedPlayerId);
        }

        const response = await axios.get(`${API_URL}/account`, {
          headers: { 'X-Player-Id': storedPlayerId }
        });
        if (!cancelled) {
          setPlayerId(storedPlayerId);
          setAccount(response.data);
          setRound(response.data.activeRound);
        }
      } catch (requestError) {
        if (!cancelled) setError(getErrorMessage(requestError));
      } finally {
        if (!cancelled) setIsBusy(false);
      }
    };

    loadAccount();
    return () => {
      cancelled = true;
    };
  }, []);

  const playerHeaders = () => ({ 'X-Player-Id': playerId });

  const updateRound = (updatedRound) => {
    setRound(updatedRound);
    setAccount((current) => current
      ? { ...current, balance: updatedRound.balance, activeRound: updatedRound.status === 'ACTIVE' ? updatedRound : null }
      : current);
  };

  const startRound = async () => {
    if (!playerId) return;
    const betAmount = Number(bet);
    if (!Number.isFinite(betAmount) || betAmount < 0.01 || betAmount > 100000) {
      setError('Der Einsatz muss zwischen CHF 0.01 und CHF 100’000 liegen.');
      return;
    }

    setIsBusy(true);
    setError('');
    try {
      const response = await axios.post(`${API_URL}/start`, { betAmount }, { headers: playerHeaders() });
      updateRound(response.data);
    } catch (requestError) {
      setError(getErrorMessage(requestError));
    } finally {
      setIsBusy(false);
    }
  };

  const takeStep = async () => {
    if (!round || !playerId) return;

    setIsBusy(true);
    setError('');
    try {
      const response = await axios.post(`${API_URL}/${round.gameId}/step`, null, { headers: playerHeaders() });
      updateRound(response.data);
    } catch (requestError) {
      setError(getErrorMessage(requestError));
    } finally {
      setIsBusy(false);
    }
  };

  const cashOut = async () => {
    if (!round || !playerId) return;

    setIsBusy(true);
    setError('');
    try {
      const response = await axios.post(`${API_URL}/${round.gameId}/cashout`, null, { headers: playerHeaders() });
      updateRound(response.data);
    } catch (requestError) {
      setError(getErrorMessage(requestError));
    } finally {
      setIsBusy(false);
    }
  };

  const active = round?.status === 'ACTIVE';
  const canStart = !active;
  const statusText = {
    LOST: 'Runde verloren',
    CASHED_OUT: 'Auszahlung gesichert',
    COMPLETED: 'Alle Felder geschafft'
  };

  return (
    <main className="game-shell">
      <header className="game-header">
        <span className="eyebrow">VIRTUELLES SPIELGUTHABEN</span>
        <h1>🐔 Chicken Road</h1>
        <p>Schaffst du es über alle 10 Felder?</p>
        <div className="balance-display" aria-live="polite">
          <span>Dein Kontostand</span>
          <strong>{account ? formatChf(account.balance) : isBusy ? 'Lädt …' : '—'}</strong>
        </div>
      </header>

      <section className="game-card" aria-label="Chicken Road Spiel">
        <div className="stake-row">
          <label htmlFor="bet">Dein Einsatz</label>
          <div className="stake-input">
            <span>CHF</span>
            <input
              id="bet"
              type="number"
              min="0.01"
              max="100000"
              step="0.01"
              value={bet}
              onChange={(event) => setBet(event.target.value)}
              disabled={active || isBusy || !playerId}
            />
          </div>
        </div>

        <div className="track" aria-label={`${round?.currentField ?? 0} von 10 Feldern sicher`}>
          {Array.from({ length: 10 }, (_, index) => {
            const isHit = round?.lastFieldResult === 'HIT' && index === round.currentField;
            const isSafe = round && index < round.currentField;
            const isNext = active && index === round.currentField;
            const className = [
              'track-field',
              isSafe && 'is-safe',
              isHit && 'is-hit',
              isNext && 'is-next'
            ].filter(Boolean).join(' ');

            return (
              <div className={className} key={index} aria-label={`Feld ${index + 1}`}>
                <span>{isHit ? '💥' : isSafe ? '🐔' : index + 1}</span>
              </div>
            );
          })}
        </div>

        <div className="round-info" aria-live="polite">
          {active ? (
            <>
              <span>Feld {round.currentField} von {round.totalFields} sicher</span>
              <strong>{round.nextFieldDangerPercent}% Gefahr auf dem nächsten Feld</strong>
            </>
          ) : round ? (
            <>
              <span>{statusText[round.status] || round.message}</span>
              <strong>{round.message}</strong>
            </>
          ) : (
            <>
              <span>10 Felder · Risiko steigt pro Schritt</span>
              <strong>Starte mit einem virtuellen Einsatz.</strong>
            </>
          )}
        </div>

        <div className="payout-panel">
          <div>
            <span>Gewinn bei Auszahlung</span>
            <strong>{formatChf(round?.profitAmount ?? 0)}</strong>
          </div>
          <div>
            <span>Gesamtauszahlung inkl. Einsatz</span>
            <strong>{formatChf(round?.cashoutAmount ?? 0)}</strong>
          </div>
        </div>

        {error && <p className="error-message" role="alert">{error}</p>}

        <div className="action-row">
          {active && (
            <>
              <button className="cashout-button" onClick={cashOut} disabled={isBusy}>
                {isBusy ? 'Einen Moment …' : 'Jetzt auszahlen'}
              </button>
              <button className="step-button" onClick={takeStep} disabled={isBusy}>
                {isBusy ? 'Huhn unterwegs …' : `Weiter zu Feld ${round.currentField + 1}`}
              </button>
            </>
          )}
          {canStart && (
            <button className="step-button start-button" onClick={startRound} disabled={isBusy || !playerId}>
              {isBusy ? 'Konto wird geladen …' : round ? 'Neue Runde starten' : 'Runde starten'}
            </button>
          )}
        </div>
      </section>

      <aside className="rules-note">
        <strong>Spielregeln</strong>
        <p>
          Die Gefahr steigt von 10 % auf Feld 1 bis 55 % auf Feld 10.
          Der Gewinn startet bei 10 % deines Einsatzes und verdoppelt sich
          nach jedem sicheren Schritt. Die Auszahlung enthält zusätzlich deinen Einsatz.
          Bei einem Treffer ist der Einsatz verloren.
        </p>
        <p>Eine Auszahlung vor dem ersten Schritt erstattet nur den Einsatz. Das virtuelle Konto ist an diesen Browser gebunden. Demo ohne echte Einzahlungen oder Auszahlungen.</p>
      </aside>
    </main>
  );
}

function getErrorMessage(error) {
  const responseMessage = error.response?.data;
  if (typeof responseMessage === 'string' && responseMessage) return responseMessage;
  if (typeof responseMessage?.error === 'string') return responseMessage.error;
  if (typeof responseMessage?.detail === 'string') return responseMessage.detail;
  if (error instanceof Error && error.message === 'Network Error') {
    return 'Der Server ist nicht erreichbar. Prüfe, ob das Backend gestartet ist.';
  }
  if (error instanceof Error && error.message) return error.message;
  return 'Der Server ist nicht erreichbar. Prüfe, ob das Backend gestartet ist.';
}

export default App;
