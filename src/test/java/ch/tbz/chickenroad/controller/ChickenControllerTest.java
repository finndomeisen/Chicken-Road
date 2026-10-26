package ch.tbz.chickenroad.controller;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.content;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import ch.tbz.chickenroad.service.ChickenGameService;
import ch.tbz.chickenroad.service.ChickenGameService.AccountResponse;
import ch.tbz.chickenroad.service.ChickenGameService.GameRoundResponse;
import ch.tbz.chickenroad.service.ChickenGameService.RoundStatus;
import ch.tbz.chickenroad.service.ChickenGameService.StepResult;
import java.math.BigDecimal;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(ChickenController.class)
class ChickenControllerTest {

    private static final String PLAYER_ID_HEADER = "X-Player-Id";

    @Autowired
    private MockMvc mockMvc;

    @MockBean
    private ChickenGameService gameService;

    @Test
    void returnsPlayerAccountAndActiveRound() throws Exception {
        UUID playerId = UUID.randomUUID();
        GameRoundResponse activeRound = round(UUID.randomUUID(), 0, RoundStatus.ACTIVE, 10);
        when(gameService.getAccount(playerId))
                .thenReturn(new AccountResponse(playerId, new BigDecimal("90.00"), activeRound));

        mockMvc.perform(get("/api/game/account").header(PLAYER_ID_HEADER, playerId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.playerId").value(playerId.toString()))
                .andExpect(jsonPath("$.balance").value(90.0))
                .andExpect(jsonPath("$.activeRound.gameId").value(activeRound.gameId().toString()));
    }

    @Test
    void startsRoundForPlayer() throws Exception {
        UUID playerId = UUID.randomUUID();
        UUID gameId = UUID.randomUUID();
        when(gameService.startRound(any(), any()))
                .thenReturn(round(gameId, 0, RoundStatus.ACTIVE, 10));

        mockMvc.perform(post("/api/game/start")
                        .header(PLAYER_ID_HEADER, playerId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"betAmount\":10}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.gameId").value(gameId.toString()))
                .andExpect(jsonPath("$.balance").value(90.0))
                .andExpect(jsonPath("$.nextFieldDangerPercent").value(10));

        verify(gameService).startRound(playerId, new BigDecimal("10"));
    }

    @Test
    void takesNextStepForPlayerRound() throws Exception {
        UUID playerId = UUID.randomUUID();
        UUID gameId = UUID.randomUUID();
        when(gameService.takeStep(playerId, gameId))
                .thenReturn(round(gameId, 1, RoundStatus.ACTIVE, 15));

        mockMvc.perform(post("/api/game/{gameId}/step", gameId).header(PLAYER_ID_HEADER, playerId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.currentField").value(1))
                .andExpect(jsonPath("$.lastFieldResult").value("SAFE"))
                .andExpect(jsonPath("$.balance").value(90.0));
    }

    @Test
    void cashesOutForPlayerAndReportsUpdatedBalance() throws Exception {
        UUID playerId = UUID.randomUUID();
        UUID gameId = UUID.randomUUID();
        when(gameService.cashOut(playerId, gameId))
                .thenReturn(round(gameId, 1, RoundStatus.CASHED_OUT, null));

        mockMvc.perform(post("/api/game/{gameId}/cashout", gameId).header(PLAYER_ID_HEADER, playerId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.status").value("CASHED_OUT"))
                .andExpect(jsonPath("$.balance").value(101.0));
    }

    @Test
    void returnsBadRequestForInvalidBet() throws Exception {
        UUID playerId = UUID.randomUUID();
        when(gameService.startRound(playerId, new BigDecimal("0")))
                .thenThrow(new IllegalArgumentException("Der Einsatz muss zwischen 0.01 und 100000.00 liegen."));

        mockMvc.perform(post("/api/game/start")
                        .header(PLAYER_ID_HEADER, playerId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"betAmount\":0}"))
                .andExpect(status().isBadRequest())
                .andExpect(content().string("Der Einsatz muss zwischen 0.01 und 100000.00 liegen."));
    }

    private GameRoundResponse round(UUID gameId, int currentField, RoundStatus status, Integer nextRisk) {
        BigDecimal bet = new BigDecimal("10.00");
        BigDecimal profit = currentField == 0 ? BigDecimal.ZERO : new BigDecimal("1.00");
        BigDecimal payout = status == RoundStatus.LOST ? BigDecimal.ZERO : bet.add(profit);
        BigDecimal balance = status == RoundStatus.CASHED_OUT ? new BigDecimal("101.00") : new BigDecimal("90.00");
        StepResult lastResult = currentField == 0 ? StepResult.NONE : StepResult.SAFE;
        return new GameRoundResponse(
                gameId, bet, currentField, 10, nextRisk, profit, payout, balance, status, lastResult, "Runde.");
    }
}
