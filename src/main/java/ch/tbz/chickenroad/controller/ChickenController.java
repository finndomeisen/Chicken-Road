package ch.tbz.chickenroad.controller;

import ch.tbz.chickenroad.service.ChickenGameService;
import ch.tbz.chickenroad.service.ChickenGameService.AccountResponse;
import ch.tbz.chickenroad.service.ChickenGameService.GameRoundResponse;
import java.math.BigDecimal;
import java.util.UUID;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.CrossOrigin;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/game")
@CrossOrigin(origins = "http://localhost:5173", allowedHeaders = {"Content-Type", "X-Player-Id"})
public class ChickenController {

    private final ChickenGameService gameService;

    public ChickenController(ChickenGameService gameService) {
        this.gameService = gameService;
    }

    @GetMapping("/account")
    public ResponseEntity<AccountResponse> getAccount(@RequestHeader("X-Player-Id") UUID playerId) {
        return ResponseEntity.ok(gameService.getAccount(playerId));
    }

    @PostMapping("/start")
    public ResponseEntity<GameRoundResponse> startRound(
            @RequestHeader("X-Player-Id") UUID playerId,
            @RequestBody StartGameRequest request) {
        return ResponseEntity.ok(gameService.startRound(playerId, request.betAmount()));
    }

    @PostMapping("/{gameId}/step")
    public ResponseEntity<GameRoundResponse> takeStep(
            @RequestHeader("X-Player-Id") UUID playerId,
            @PathVariable UUID gameId) {
        return ResponseEntity.ok(gameService.takeStep(playerId, gameId));
    }

    @PostMapping("/{gameId}/cashout")
    public ResponseEntity<GameRoundResponse> cashOut(
            @RequestHeader("X-Player-Id") UUID playerId,
            @PathVariable UUID gameId) {
        return ResponseEntity.ok(gameService.cashOut(playerId, gameId));
    }

    @ExceptionHandler(IllegalArgumentException.class)
    public ResponseEntity<String> handleInvalidBet(IllegalArgumentException exception) {
        return ResponseEntity.badRequest().body(exception.getMessage());
    }

    public record StartGameRequest(BigDecimal betAmount) {
    }
}