--
-- TABLE: chains
--
CREATE TABLE public.chains (
    id text NOT NULL,
    rpc text NOT NULL,
    ws text NOT NULL
);

ALTER TABLE ONLY public.chains 
    ADD CONSTRAINT chains_pkey PRIMARY KEY (id);

--
-- TABLE: projects
--
CREATE TABLE public.projects (
    id text NOT NULL,
    name text NOT NULL,
    chain text NOT NULL,
    status text NOT NULL,
    secret text NOT NULL,
    create_time timestamp WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.projects
    ADD CONSTRAINT chain_id_fk FOREIGN KEY (chain) REFERENCES public.chains(id) ON DELETE CASCADE;
