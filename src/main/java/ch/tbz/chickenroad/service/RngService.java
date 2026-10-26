package ch.tbz.chickenroad.service;

import org.springframework.stereotype.Service;
import java.security.SecureRandom;

@Service
public class RngService {
    private final SecureRandom random = new SecureRandom();

    public double nextDouble() {
        return random.nextDouble();
    }
}
