-- Add an explicit project namespace to mutation proposals so capability policy
-- can constrain path, project, and durable session independently.
ALTER TABLE mutation_proposals ADD COLUMN project TEXT;

CREATE INDEX IF NOT EXISTS idx_mutation_proposals_project
  ON mutation_proposals(project, updated_at DESC);
