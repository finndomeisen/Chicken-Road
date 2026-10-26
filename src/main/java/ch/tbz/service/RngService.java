package ch.tbz.service;

import org.springframework.stereotype.Service;
import java.util.Random;

@Service
public class RngService {
    private final Random random = new Random();

    // Gibt true zurück, wenn das Huhn überlebt (50% Chance)
    public boolean doesChickenSurvive() {
        return random.nextBoolean();
    }
}