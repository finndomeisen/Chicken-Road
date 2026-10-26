# Chicken Road

Ein Fullstack-Projekt für das Modul M450

Ein Gambling Spiel, bei dem Spieler Einsätze platzieren und darauf wetten, ob das Huhn sicher die Strasse überquert.

## 🏗️ Grobe Architektur
* **Frontend:** React (UI für Wetteinsätze, Kontostand und visuelle Rückmeldung).
* **Backend:** Java Spring Boot (Verwaltung der Wetteinsätze, Validierung des Kontostands, Gambling-Logik).
* **Kommunikation:** REST-API.
* **Infrastruktur:** Docker Compose für lokales Setup, GitHub Actions als CI-Pipeline.

## 🤖 KI-Nutzung im Projekt
Wir haben KI-Werkzeuge (LLMs) in folgendem Rahmen genutzt:
* Als Nachschlagewerk für Detailfragen zu Spring Boot Annotations und React-Hooks.
* Als Assistenz (Autocomplete) beim Schreiben der Unit-Tests.
Wir können den gesamten Code erklären, anpassen und manuell erweitern.

## 🧪 Testkonzept & Strategie
* **Unit Tests:** Die Core-Gambling-Logik (`ChickenGameService`) wird zu 100% mit Unit-Tests abgedeckt.
* **Mocking:** Der Zufallsgenerator (`RngService`), der über Sieg oder Niederlage entscheidet, wird in den Tests mit Mockito weggemockt. Dadurch können wir deterministische Tests für "Gewinn" und "Verlust" schreiben, ohne auf echten Zufall angewiesen zu sein.
* **Integration Tests:** Wir testen das Zusammenspiel zwischen dem REST-Controller und dem Service (via `@WebMvcTest` und MockMvc).
* **Automatisierung:** Bei jedem Push auf `main` führt unsere GitHub Actions Pipeline automatisch alle Tests aus und generiert einen JaCoCo-Report.

## 🔄 TDD & Code Review Reflexion (Fazit)
* **TDD:** Die Anwendung von Test-Driven Development (Red-Green-Refactor) hat uns geholfen, die Wetteinsatz-Logik von Anfang an modular aufzubauen. Das Auslagern des Zufallsgenerators in einen eigenen Service war eine direkte Konsequenz davon, um den Code überhaupt testbar zu machen.
* **Code Reviews:** Die Regel, dass PRs aktiv kommentiert und gechallenged werden müssen, hat Flüchtigkeitsfehler bei der Validierung von negativen Wetteinsätzen verhindert.
