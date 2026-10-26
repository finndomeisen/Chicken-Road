import React, { useState } from 'react';
import axios from 'axios';

function App() {
  const [bet, setBet] = useState(10);
  const [message, setMessage] = useState('Setze deinen Einsatz!');
  const [status, setStatus] = useState('idle');

  const handlePlay = async () => {
    if (bet <= 0) {
      setMessage('Der Einsatz muss grösser als 0 sein!');
      return;
    }

    try {
      // POST-Request an das Spring Boot Backend
      const response = await axios.post(`http://localhost:8080/api/game/play?betAmount=${bet}`);
      const winAmount = response.data;

      if (winAmount > 0) {
        setMessage(`🎉 Gewonnen! Du hast ${winAmount} CHF abgeräumt!`);
        setStatus('win');
      } else {
        setMessage('💀 Verloren! Überfahren. Dein Einsatz ist weg.');
        setStatus('loss');
      }
    } catch (error) {
      setMessage('Fehler bei der Verbindung zum Server.');
      console.error(error);
    }
  };

  return (
    <div style={{ textAlign: 'center', marginTop: '50px', fontFamily: 'sans-serif' }}>
      <h1>🐔 Chicken Road (oder was auch immer es wird)</h1>
      <h2 style={{ color: status === 'win' ? 'green' : status === 'loss' ? 'red' : 'white' }}>
        {message}
      </h2>
      
      <div style={{ margin: '30px' }}>
        <label style={{ fontSize: '18px', marginRight: '10px' }}>Einsatz (CHF):</label>
        <input 
          type="number" 
          value={bet} 
          onChange={(e) => setBet(e.target.value)} 
          style={{ fontSize: '18px', padding: '5px', width: '100px', textAlign: 'center' }}
        />
      </div>

      <button 
        onClick={handlePlay} 
        style={{ 
          fontSize: '20px', 
          padding: '15px 30px', 
          cursor: 'pointer', 
          backgroundColor: '#bf1650', 
          color: 'white', 
          border: 'none', 
          borderRadius: '5px',
          textTransform: 'uppercase',
          letterSpacing: '2px'
        }}
      >
        Über die Strasse rennen
      </button>
    </div>
  );
}

export default App;
