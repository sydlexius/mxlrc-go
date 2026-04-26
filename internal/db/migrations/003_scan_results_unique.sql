-- +goose Up
-- +goose StatementBegin
DELETE FROM scan_results
WHERE id NOT IN (
    SELECT MAX(id)
    FROM scan_results
    GROUP BY library_id, file_path
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_scan_results_library_file
    ON scan_results(library_id, file_path);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_scan_results_library_file;
-- +goose StatementEnd
