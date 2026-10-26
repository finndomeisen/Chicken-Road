package ch.tbz.chickenroad.repository;

import ch.tbz.chickenroad.model.PlayerAccount;
import jakarta.persistence.LockModeType;
import java.util.Optional;
import java.util.UUID;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

public interface PlayerAccountRepository extends JpaRepository<PlayerAccount, UUID> {

    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select account from PlayerAccount account where account.id = :id")
    Optional<PlayerAccount> findByIdForUpdate(@Param("id") UUID id);
}
