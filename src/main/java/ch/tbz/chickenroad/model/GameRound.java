package ch.tbz.chickenroad.model;

import ch.tbz.chickenroad.service.ChickenGameService.RoundStatus;
import ch.tbz.chickenroad.service.ChickenGameService.StepResult;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import jakarta.persistence.Version;
import java.math.BigDecimal;
import java.util.UUID;

@Entity
@Table(name = "game_rounds")
public class GameRound {

    @Id
    private UUID id;

    @Column(nullable = false)
    private UUID playerId;

    @Column(nullable = false, precision = 19, scale = 2)
    private BigDecimal betAmount;

    @Column(nullable = false)
    private int currentField;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 16)
    private RoundStatus status;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 8)
    private StepResult lastFieldResult;

    @Version
    private long version;

    protected GameRound() {
    }

    public GameRound(UUID id, UUID playerId, BigDecimal betAmount) {
        this.id = id;
        this.playerId = playerId;
        this.betAmount = betAmount;
        this.status = RoundStatus.ACTIVE;
        this.lastFieldResult = StepResult.NONE;
    }

    public UUID getId() {
        return id;
    }

    public UUID getPlayerId() {
        return playerId;
    }

    public BigDecimal getBetAmount() {
        return betAmount;
    }

    public int getCurrentField() {
        return currentField;
    }

    public void setCurrentField(int currentField) {
        this.currentField = currentField;
    }

    public RoundStatus getStatus() {
        return status;
    }

    public void setStatus(RoundStatus status) {
        this.status = status;
    }

    public StepResult getLastFieldResult() {
        return lastFieldResult;
    }

    public void setLastFieldResult(StepResult lastFieldResult) {
        this.lastFieldResult = lastFieldResult;
    }
}
