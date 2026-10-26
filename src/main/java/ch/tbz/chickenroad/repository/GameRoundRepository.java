package ch.tbz.chickenroad.repository;

import ch.tbz.chickenroad.model.GameRound;
import ch.tbz.chickenroad.service.ChickenGameService.RoundStatus;
import jakarta.persistence.LockModeType;
import java.util.Optional;
import java.util.UUID;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

public interface GameRoundRepository extends JpaRepository<GameRound, UUID> {

    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select round from GameRound round where round.id = :id and round.playerId = :playerId")
    Optional<GameRound> findByIdAndPlayerIdForUpdate(
            @Param("id") UUID id,
            @Param("playerId") UUID playerId);

    boolean existsByPlayerIdAndStatus(UUID playerId, RoundStatus status);

    Optional<GameRound> findFirstByPlayerIdAndStatusOrderById(UUID playerId, RoundStatus status);
}
