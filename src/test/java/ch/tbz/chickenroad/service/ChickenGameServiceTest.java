package ch.tbz.chickenroad.service;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.when;

import ch.tbz.chickenroad.model.PlayerAccount;
import ch.tbz.chickenroad.repository.PlayerAccountRepository;
import ch.tbz.chickenroad.service.ChickenGameService.AccountResponse;
import ch.tbz.chickenroad.service.ChickenGameService.GameRoundResponse;
import ch.tbz.chickenroad.service.ChickenGameService.RoundStatus;
import ch.tbz.chickenroad.service.ChickenGameService.StepResult;
import jakarta.persistence.EntityManager;
import java.math.BigDecimal;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.web.server.ResponseStatusException;

@SpringBootTest(properties = {
    "spring.datasource.url=jdbc:h2:mem:chicken-road-test;DB_CLOSE_DELAY=-1",
    "spring.jpa.hibernate.ddl-auto=create-drop"
})
class ChickenGameServiceTest {

    @Autowired
    private ChickenGameService gameService;

    @Autowired
    private PlayerAccountRepository accountRepository;

    @Autowired
    private EntityManager entityManager;

    @MockBean
    private RngService rngService;

    @BeforeEach
    void resetRandomGenerator() {
        reset(rngService);
    }

    @Test
    void createsAndPersistsStartingBalanceForEachPlayer() {
        UUID playerId = UUID.randomUUID();

        AccountResponse account = gameService.getAccount(playerId);
        entityManager.clear();
        PlayerAccount storedAccount = accountRepository.findById(playerId).orElseThrow();

        assertEquals(ChickenGameService.INITIAL_BALANCE, account.balance());
        assertEquals(ChickenGameService.INITIAL_BALANCE, storedAccount.getBalance());
        assertNull(account.activeRound());
    }

    @Test
    void raisesDangerByFivePercentagePointsOnEveryField() {
        for (int field = 0; field < ChickenGameService.TOTAL_FIELDS; field++) {
            assertEquals(10 + field * 5, ChickenGameService.dangerPercentForField(field));
        }
        assertThrows(IllegalArgumentException.class, () -> ChickenGameService.dangerPercentForField(10));
    }

    @Test
    void deductsStakeAndCreditsCashoutExactlyOnce() {
        UUID playerId = UUID.randomUUID();
        when(rngService.nextDouble()).thenReturn(0.99, 0.90);
        GameRoundResponse round = gameService.startRound(playerId, new BigDecimal("10.00"));

        assertEquals(new BigDecimal("90.00"), round.balance());
        round = gameService.takeStep(playerId, round.gameId());
        assertEquals(1, round.currentField());
        assertEquals(15, round.nextFieldDangerPercent());
        assertEquals(new BigDecimal("1.00"), round.profitAmount());
        assertEquals(new BigDecimal("11.00"), round.cashoutAmount());
        assertEquals(new BigDecimal("90.00"), round.balance());

        round = gameService.takeStep(playerId, round.gameId());
        assertEquals(2, round.currentField());
        assertEquals(20, round.nextFieldDangerPercent());
        assertEquals(new BigDecimal("2.00"), round.profitAmount());
        assertEquals(new BigDecimal("12.00"), round.cashoutAmount());

        round = gameService.cashOut(playerId, round.gameId());
        assertEquals(RoundStatus.CASHED_OUT, round.status());
        assertEquals(new BigDecimal("102.00"), round.balance());
        UUID gameId = round.gameId();
        assertThrows(IllegalArgumentException.class, () -> gameService.cashOut(playerId, gameId));

        entityManager.clear();
        assertEquals(new BigDecimal("102.00"), accountRepository.findById(playerId).orElseThrow().getBalance());
    }

    @Test
    void cashoutBeforeTheFirstStepRefundsTheStake() {
        UUID playerId = UUID.randomUUID();
        GameRoundResponse round = gameService.startRound(playerId, new BigDecimal("10.00"));
        assertEquals(new BigDecimal("90.00"), round.balance());

        round = gameService.cashOut(playerId, round.gameId());

        assertEquals(RoundStatus.CASHED_OUT, round.status());
        assertEquals(BigDecimal.ZERO.setScale(2), round.profitAmount());
        assertEquals(new BigDecimal("10.00"), round.cashoutAmount());
        assertEquals(ChickenGameService.INITIAL_BALANCE, round.balance());
    }

    @Test
    void restoresActiveRoundAndBalanceAfterReloadingAccount() {
        UUID playerId = UUID.randomUUID();
        GameRoundResponse started = gameService.startRound(playerId, new BigDecimal("10.00"));
        entityManager.clear();

        AccountResponse reloaded = gameService.getAccount(playerId);

        assertEquals(new BigDecimal("90.00"), reloaded.balance());
        assertEquals(started.gameId(), reloaded.activeRound().gameId());
        assertEquals(RoundStatus.ACTIVE, reloaded.activeRound().status());
    }

    @Test
    void losesStakeOnHitAndClearsUnsecuredProfit() {
        UUID playerId = UUID.randomUUID();
        when(rngService.nextDouble()).thenReturn(0.99, 0.10);
        GameRoundResponse round = gameService.startRound(playerId, new BigDecimal("10.00"));
        round = gameService.takeStep(playerId, round.gameId());

        round = gameService.takeStep(playerId, round.gameId());

        assertEquals(RoundStatus.LOST, round.status());
        assertEquals(StepResult.HIT, round.lastFieldResult());
        assertEquals(BigDecimal.ZERO.setScale(2), round.profitAmount());
        assertEquals(BigDecimal.ZERO.setScale(2), round.cashoutAmount());
        assertEquals(new BigDecimal("90.00"), round.balance());
    }

    @Test
    void creditsPayoutAutomaticallyAfterCompletingEveryField() {
        UUID playerId = UUID.randomUUID();
        when(rngService.nextDouble()).thenReturn(0.99);
        GameRoundResponse round = gameService.startRound(playerId, new BigDecimal("10.00"));

        for (int field = 0; field < ChickenGameService.TOTAL_FIELDS; field++) {
            round = gameService.takeStep(playerId, round.gameId());
        }

        assertEquals(RoundStatus.COMPLETED, round.status());
        assertEquals(10, round.currentField());
        assertEquals(new BigDecimal("522.00"), round.cashoutAmount());
        assertEquals(new BigDecimal("612.00"), round.balance());
        assertNull(round.nextFieldDangerPercent());
    }

    @Test
    void preventsOverspendingAndMultipleActiveRounds() {
        UUID playerId = UUID.randomUUID();
        gameService.getAccount(playerId);

        assertThrows(
                IllegalArgumentException.class,
                () -> gameService.startRound(playerId, new BigDecimal("100.01")));

        GameRoundResponse activeRound = gameService.startRound(playerId, new BigDecimal("10.00"));
        assertThrows(
                IllegalArgumentException.class,
                () -> gameService.startRound(playerId, new BigDecimal("1.00")));
        assertEquals(new BigDecimal("90.00"), gameService.getAccount(playerId).balance());
        assertEquals(activeRound.gameId(), gameService.getAccount(playerId).activeRound().gameId());
    }

    @Test
    void rejectsInvalidBetsAndUnknownOrForeignRounds() {
        UUID playerId = UUID.randomUUID();
        for (BigDecimal bet : new BigDecimal[] {
                null, new BigDecimal("0"), new BigDecimal("-1"),
                new BigDecimal("0.001"), new BigDecimal("100000.01")}) {
            assertThrows(IllegalArgumentException.class, () -> gameService.startRound(playerId, bet));
        }

        assertThrows(
                ResponseStatusException.class,
                () -> gameService.takeStep(playerId, UUID.randomUUID()));

        UUID otherPlayerId = UUID.randomUUID();
        GameRoundResponse round = gameService.startRound(playerId, new BigDecimal("1.00"));
        assertThrows(
                ResponseStatusException.class,
                () -> gameService.takeStep(otherPlayerId, round.gameId()));
    }
}
