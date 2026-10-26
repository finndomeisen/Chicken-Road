package src.main.java.ch.tbz.controller;

import ch.tbz.chickenroad.service.ChickenGameService;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/game")
@CrossOrigin(origins = "http://localhost:3000") // Erlaubt Anfragen vom React-Frontend
public class ChickenController {
    
    private final ChickenRoadService gameService;

    public ChickenController(ChickenRoadService gameService) {
        this.gameService = gameService;
    }

    @PostMapping("/play")
    public ResponseEntity<Double> playRound(@RequestParam double betAmount) {
        try {
            double result = gameService.playRound(betAmount);
            return ResponseEntity.ok(result);
        } catch (IllegalArgumentException e) {
            return ResponseEntity.badRequest().body(-1.0);
        }
    }
}