CREATE TABLE payments (
    id BIGSERIAL PRIMARY KEY,

    reference VARCHAR(64) NOT NULL UNIQUE,

    amount BIGINT NOT NULL CHECK (amount > 0),

    currency CHAR(3) NOT NULL,

    status VARCHAR(20) NOT NULL
        CHECK (status IN (
            'pending',
            'succeeded',
            'failed',
            'cancelled'
        )),

    provider VARCHAR(30) NOT NULL,

    provider_reference VARCHAR(100) UNIQUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_status ON payments(status);
