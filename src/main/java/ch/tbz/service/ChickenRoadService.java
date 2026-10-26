package ch.tbz.chickenroad.service;

import org.springframework.stereotype.Service;

@Service
public class ChickenRoadService {
    
    private final RngService rngService;

    // Dependency Injection: Macht den Code testbar, da wir den RngService austauschen können
    public ChickenRoadService(RngService rngService) {
        this.rngService = rngService;
    }

    public double playRound(double betAmount) {
        if (betAmount <= 0) {
            throw new IllegalArgumentException("Einsatz muss grösser als 0 sein.");
        }

        if (rngService.doesChickenSurvive()) {
            return betAmount * 2.0; // Gewinn: Einsatz wird verdoppelt
        } else {
            return 0.0; // Verlust: Einsatz ist weg
        }
    }
}