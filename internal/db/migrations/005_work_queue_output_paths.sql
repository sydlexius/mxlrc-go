-- +goose Up
-- +goose StatementBegin
ALTER TABLE work_queue ADD COLUMN output_paths TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE work_queue DROP COLUMN output_paths;
-- +goose StatementEnd
