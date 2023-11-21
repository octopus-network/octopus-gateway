-- Setting default value to empty string
ALTER TABLE public.chains ADD COLUMN eth_rpc text NOT NULL DEFAULT '';
ALTER TABLE public.chains ADD COLUMN eth_ws text NOT NULL DEFAULT '';
