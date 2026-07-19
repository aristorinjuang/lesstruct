-- Add content format column (tiptap, markdown, html)
ALTER TABLE content_items ADD COLUMN format VARCHAR(16) NOT NULL DEFAULT 'tiptap';
