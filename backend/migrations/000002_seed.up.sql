CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Test user: test@example.com / password123 (bcrypt via pgcrypto, cost 12)
INSERT INTO users (id, name, email, password_hash)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'Test User',
    'test@example.com',
    crypt('password123', gen_salt('bf', 12))
);

INSERT INTO projects (id, name, description, owner_id)
VALUES (
    'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12',
    'Sample Project',
    'Seeded project for reviewers',
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'
);

INSERT INTO tasks (id, title, description, status, priority, project_id, assignee_id, created_by, due_date)
VALUES
    (
        'c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01',
        'Draft landing page copy',
        'First iteration of marketing copy',
        'todo',
        'medium',
        'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12',
        NULL,
        'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
        CURRENT_DATE + 7
    ),
    (
        'c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02',
        'Implement auth middleware',
        'JWT validation and error mapping',
        'in_progress',
        'high',
        'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12',
        'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
        'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
        CURRENT_DATE + 3
    ),
    (
        'c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a03',
        'Write README',
        'Overview, setup, architecture notes',
        'done',
        'low',
        'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12',
        'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
        'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
        CURRENT_DATE - 1
    );
