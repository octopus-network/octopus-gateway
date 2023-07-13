-- Setting default value to empty string
ALTER TABLE public.chains ALTER COLUMN ws SET DEFAULT '';
ALTER TABLE public.chains ADD COLUMN grpc text NOT NULL DEFAULT '';
