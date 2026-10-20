ALTER TABLE accounts
    ADD COLUMN display_name TEXT NOT NULL DEFAULT ''
        CHECK (char_length(display_name) <= 160);
