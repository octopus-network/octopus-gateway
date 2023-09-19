-- Setting default value to empty string
ALTER TABLE public.chains ADD COLUMN rest text NOT NULL DEFAULT '';
