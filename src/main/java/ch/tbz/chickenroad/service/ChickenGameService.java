package ch.tbz.chickenroad.service;

import ch.tbz.chickenroad.model.GameRound;
import ch.tbz.chickenroad.model.PlayerAccount;
import ch.tbz.chickenroad.repository.GameRoundRepository;
import ch.tbz.chickenroad.repository.PlayerAccountRepository;
import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.UUID;
import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.server.ResponseStatusException;

@Service
public class ChickenGameService {

    public static final int TOTAL_FIELDS = 10;
    public static final BigDecimal INITIAL_BALANCE = new BigDecimal("100.00");
    private static final int FIRST_FIELD_DANGER_PERCENT = 10;
    private static final int DANGER_INCREASE_PER_FIELD_PERCENT = 5;
    private static final BigDecimal FIRST_FIELD_PROFIT_RATE = new BigDecimal("0.10");
    private static final BigDecimal MINIMUM_BET = new BigDecimal("0.01");
    private static final BigDecimal MAXIMUM_BET = new BigDecimal("100000.00");

    private final RngService rngService;
    private final PlayerAccountRepository accountRepository;
    private final GameRoundRepository roundRepository;

    public ChickenGameService(
            RngService rngService,
            PlayerAccountRepository accountRepository,
            GameRoundRepository roundRepository) {
        this.rngService = rngService;
        this.accountRepository = accountRepository;
        this.roundRepository = roundRepository;
    }

    @Transactional
    public AccountResponse getAccount(UUID playerId) {
        PlayerAccount account = getOrCreateAccount(playerId);
        GameRound activeRound = roundRepository
                .findFirstByPlayerIdAndStatusOrderById(playerId, RoundStatus.ACTIVE)
                .orElse(null);
        GameRoundResponse activeRoundResponse = activeRound == null
                ? null
                : toResponse(activeRound, account.getBalance(), "Deine Runde läuft weiter.");
        return new AccountResponse(playerId, account.getBalance(), activeRoundResponse);
    }

    @Transactional
    public GameRoundResponse startRound(UUID playerId, BigDecimal betAmount) {
        validateBet(betAmount);
        PlayerAccount account = getOrCreateAccount(playerId);
        if (roundRepository.existsByPlayerIdAndStatus(playerId, RoundStatus.ACTIVE)) {
            throw new IllegalArgumentException("Du hast bereits eine laufende Runde.");
        }

        BigDecimal normalizedBet = betAmount.setScale(2, RoundingMode.UNNECESSARY);
        if (account.getBalance().compareTo(normalizedBet) < 0) {
            throw new IllegalArgumentException("Dein Guthaben reicht für diesen Einsatz nicht aus.");
        }

        account.setBalance(account.getBalance().subtract(normalizedBet));
        accountRepository.save(account);

        GameRound round = new GameRound(UUID.randomUUID(), playerId, normalizedBet);
        roundRepository.save(round);
        return toResponse(
                round,
                account.getBalance(),
                "Runde gestartet. Wage deinen ersten Schritt oder zahle jederzeit aus.");
    }

    @Transactional
    public GameRoundResponse takeStep(UUID playerId, UUID roundId) {
        GameRound round = findRound(playerId, roundId);
        ensureActive(round);
        if (round.getCurrentField() == TOTAL_FIELDS) {
            throw new IllegalArgumentException("Das letzte Feld wurde bereits erreicht.");
        }

        int dangerPercent = dangerPercentForField(round.getCurrentField());
        boolean hit = rngService.nextDouble() < dangerPercent / 100.0;
        round.setLastFieldResult(hit ? StepResult.HIT : StepResult.SAFE);

        PlayerAccount account;
        String message;
        if (hit) {
            round.setStatus(RoundStatus.LOST);
            account = getAccountForUpdate(playerId);
            message = "Das Huhn wurde auf Feld " + (round.getCurrentField() + 1) + " überfahren. Der Einsatz ist verloren.";
        } else {
            round.setCurrentField(round.getCurrentField() + 1);
            if (round.getCurrentField() == TOTAL_FIELDS) {
                round.setStatus(RoundStatus.COMPLETED);
                account = getAccountForUpdate(playerId);
                account.setBalance(account.getBalance().add(cashoutAmount(round)));
                accountRepository.save(account);
                message = "Alle 10 Felder geschafft! Die Auszahlung wurde deinem Guthaben gutgeschrieben.";
            } else {
                account = accountRepository.findById(playerId)
                        .orElseThrow(this::accountNotFound);
                message = "Sicher angekommen! Du kannst weitergehen oder jetzt auszahlen.";
            }
        }

        roundRepository.save(round);
        return toResponse(round, account.getBalance(), message);
    }

    @Transactional
    public GameRoundResponse cashOut(UUID playerId, UUID roundId) {
        GameRound round = findRound(playerId, roundId);
        ensureActive(round);
        PlayerAccount account = getAccountForUpdate(playerId);

        round.setStatus(RoundStatus.CASHED_OUT);
        account.setBalance(account.getBalance().add(cashoutAmount(round)));
        accountRepository.save(account);
        roundRepository.save(round);
        return toResponse(round, account.getBalance(), "Auszahlung deinem Guthaben gutgeschrieben.");
    }

    public static int dangerPercentForField(int zeroBasedField) {
        if (zeroBasedField < 0 || zeroBasedField >= TOTAL_FIELDS) {
            throw new IllegalArgumentException("Das Feld muss zwischen 1 und 10 liegen.");
        }
        return FIRST_FIELD_DANGER_PERCENT + DANGER_INCREASE_PER_FIELD_PERCENT * zeroBasedField;
    }

    private void validateBet(BigDecimal betAmount) {
        if (betAmount == null
                || betAmount.compareTo(MINIMUM_BET) < 0
                || betAmount.compareTo(MAXIMUM_BET) > 0
                || betAmount.scale() > 2) {
            throw new IllegalArgumentException("Der Einsatz muss zwischen 0.01 und 100000.00 liegen (maximal 2 Dezimalstellen).");
        }
    }

    private PlayerAccount getOrCreateAccount(UUID playerId) {
        if (playerId == null) {
            throw new IllegalArgumentException("Die Spieler-ID fehlt.");
        }
        return accountRepository.findByIdForUpdate(playerId)
                .orElseGet(() -> accountRepository.save(new PlayerAccount(playerId, INITIAL_BALANCE)));
    }

    private PlayerAccount getAccountForUpdate(UUID playerId) {
        return accountRepository.findByIdForUpdate(playerId)
                .orElseThrow(this::accountNotFound);
    }

    private GameRound findRound(UUID playerId, UUID roundId) {
        return roundRepository.findByIdAndPlayerIdForUpdate(roundId, playerId)
                .orElseThrow(() -> new ResponseStatusException(HttpStatus.NOT_FOUND, "Spielrunde nicht gefunden."));
    }

    private ResponseStatusException accountNotFound() {
        return new ResponseStatusException(HttpStatus.NOT_FOUND, "Spielerkonto nicht gefunden.");
    }

    private void ensureActive(GameRound round) {
        if (round.getStatus() != RoundStatus.ACTIVE) {
            throw new IllegalArgumentException("Diese Spielrunde ist bereits beendet.");
        }
    }

    private BigDecimal cashoutAmount(GameRound round) {
        return round.getBetAmount().add(profitAmount(round)).setScale(2, RoundingMode.HALF_UP);
    }

    private BigDecimal profitAmount(GameRound round) {
        if (round.getStatus() == RoundStatus.LOST || round.getCurrentField() == 0) {
            return BigDecimal.ZERO.setScale(2);
        }
        return round.getBetAmount()
                .multiply(FIRST_FIELD_PROFIT_RATE)
                .multiply(BigDecimal.valueOf(2).pow(round.getCurrentField() - 1))
                .setScale(2, RoundingMode.HALF_UP);
    }

    private GameRoundResponse toResponse(GameRound round, BigDecimal balance, String message) {
        Integer nextDangerPercent = round.getStatus() == RoundStatus.ACTIVE && round.getCurrentField() < TOTAL_FIELDS
                ? dangerPercentForField(round.getCurrentField())
                : null;

        return new GameRoundResponse(
                round.getId(),
                round.getBetAmount(),
                round.getCurrentField(),
                TOTAL_FIELDS,
                nextDangerPercent,
                profitAmount(round),
                round.getStatus() == RoundStatus.LOST ? BigDecimal.ZERO.setScale(2) : cashoutAmount(round),
                balance,
                round.getStatus(),
                round.getLastFieldResult(),
                message);
    }

    public enum RoundStatus {
        ACTIVE,
        LOST,
        CASHED_OUT,
        COMPLETED
    }

    public enum StepResult {
        NONE,
        SAFE,
        HIT
    }

    public record AccountResponse(UUID playerId, BigDecimal balance, GameRoundResponse activeRound) {
    }

    public record GameRoundResponse(
            UUID gameId,
            BigDecimal betAmount,
            int currentField,
            int totalFields,
            Integer nextFieldDangerPercent,
            BigDecimal profitAmount,
            BigDecimal cashoutAmount,
            BigDecimal balance,
            RoundStatus status,
            StepResult lastFieldResult,
            String message) {
    }
}
