# Chicken Road

Ein Fullstack-Projekt für das Modul M450

Ein Gambling Spiel, bei dem Spieler Einsätze platzieren und darauf wetten, ob das Huhn sicher die Strasse überquert.

## 🏗️ Architektur
* **Frontend:** React mit Vite (Einsatz und Spielausgang).
* **Backend:** Java 17 und Spring Boot (Spiel-Logik, Einsatzvalidierung und Kontostand).
* **Datenbank:** Lokale H2-Datei für Spielerkonten und laufende/beendete Runden.
* **Kommunikation:** REST-API mit Rundenstart, Einzelschritt und Auszahlung.
* **CI:** GitHub Actions baut und testet Backend und Frontend.

## 🎮 Spielregeln der Demo
* Eine Runde hat 10 Felder. Das Überfahr-Risiko des nächsten Feldes steigt von 10 % bis 55 % in 5-Prozentpunktschritten.
* Nach jedem sicheren Schritt kann weitergespielt oder ausgezahlt werden.
* Der Gewinn (zusätzlich zum Einsatz) beginnt bei 10 % des Einsatzes und verdoppelt sich pro sicherem Feld: `Einsatz × 0.10 × 2^(sichere Felder − 1)`.
* Ein Treffer verliert den Einsatz. Das Erreichen des zehnten Feldes zahlt automatisch aus.
* Guthaben und Runden sind nur virtuell. Es gibt keine echten Einzahlungen oder Auszahlungen.
* Jedes Browserprofil erhält eine eigene Spieler-ID und CHF 100 Startguthaben. Das Guthaben und Spielrunden liegen in `data/chicken-road.mv.db` und bleiben bei einem Backend-Neustart erhalten.
* Die Spieler-ID liegt im Browser-`localStorage`; Browserdaten löschen oder ein anderer Browser bedeutet deshalb ein anderes Konto. Das ist keine Anmeldung und kein Schutz vor Manipulation.

Diese Regeln sind eine eigene Demo-Variante, keine exakte Kopie der verlinkten 25-Felder-Varianten. Als Recherche zur allgemeinen Chicken-Road-Mechanik (Feld für Feld, steigender Multiplikator und Cash-out) diente die [Chicken-Road-Anleitung](https://chickenroad.ch/de-ch/anleitung/).

## 🚀 Lokal starten
Voraussetzungen: Java 17, Maven und Node.js 20 oder neuer.

Backend im Projektverzeichnis starten:
```bash
mvn spring-boot:run
```

Frontend in einem zweiten Terminal starten:
```bash
npm ci
npm run dev --workspace frontend
```

Das Frontend ist anschliessend unter `http://localhost:5173` erreichbar.

## 🧪 Tests und Build
```bash
mvn clean verify
npm ci
npm run build
```

## 🤖 KI-Nutzung im Projekt
Wir haben KI-Werkzeuge (LLMs) in folgendem Rahmen genutzt:
* Als Nachschlagewerk für Detailfragen zu Spring Boot Annotations und React-Hooks.
* Als Assistenz (Autocomplete) beim Schreiben der Unit-Tests.
Wir können den gesamten Code erklären, anpassen und manuell erweitern.

## 🧪 Testkonzept & Strategie
* **Unit- und Integrationstests:** Die Rundenlogik und Datenbank testen sichere Schritte, steigendes Risiko, Auszahlung, Treffer, Guthabenänderungen und das Wiederherstellen laufender Runden.
* **Mocking:** Der Zufallsgenerator (`RngService`) wird in Tests ersetzt, damit die Spielausgänge reproduzierbar sind.
* **Automatisierung:** Bei Pull Requests und Pushes auf `main` testet die GitHub-Actions-Pipeline das Backend und baut das Frontend.

## 🔄 TDD & Code Review Reflexion (Fazit)
* **TDD:** Die Anwendung von Test-Driven Development (Red-Green-Refactor) hat uns geholfen, die Wetteinsatz-Logik von Anfang an modular aufzubauen. Das Auslagern des Zufallsgenerators in einen eigenen Service war eine direkte Konsequenz davon, um den Code überhaupt testbar zu machen.
* **Code Reviews:** Die Regel, dass PRs aktiv kommentiert und gechallenged werden müssen, hat Flüchtigkeitsfehler bei der Validierung von negativen Wetteinsätzen verhindert.
