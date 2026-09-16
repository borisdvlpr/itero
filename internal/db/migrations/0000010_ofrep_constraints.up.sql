ALTER TABLE flags
    ADD CONSTRAINT flags_metadata_is_flat_scalars CHECK (
        NOT jsonb_path_exists(
                metadata,
                'strict $.* ? (@.type() != "string" && @.type() != "number" && @.type() != "boolean")'
            )
        );

COMMENT
ON COLUMN flags.metadata IS
    'Returned verbatim as the OFREP flag metadata object. Values must be flat '
    'scalars: boolean, string or number.';
