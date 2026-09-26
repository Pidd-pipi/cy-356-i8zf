-- 002_plot_custodians: 地块共管功能
-- 与 database/init.sql 保持一致；GORM AutoMigrate 为运行时权威 schema。
CREATE TABLE IF NOT EXISTS plot_custodians (
    id BIGSERIAL PRIMARY KEY,
    plot_id BIGINT NOT NULL REFERENCES plots(id),
    custodian_id BIGINT NOT NULL REFERENCES users(id),
    inviter_id BIGINT NOT NULL REFERENCES users(id),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    invited_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    accepted_at TIMESTAMPTZ,
    removed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_plot_custodians_plot ON plot_custodians(plot_id);
CREATE INDEX IF NOT EXISTS idx_plot_custodians_custodian ON plot_custodians(custodian_id);
CREATE INDEX IF NOT EXISTS idx_plot_custodians_status ON plot_custodians(status);
