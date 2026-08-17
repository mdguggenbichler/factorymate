-- Default embed footers included {Timestamp} while show_timestamp also enabled Discord's
-- native footer timestamp, producing duplicate times. Strip the redundant placeholder.

UPDATE message_types
SET default_template_json = REPLACE(default_template_json, ' · {Timestamp}', '')
WHERE default_template_json LIKE '% · {Timestamp}%';

UPDATE message_templates
SET template_json = REPLACE(template_json, ' · {Timestamp}', '')
WHERE template_json LIKE '% · {Timestamp}%';
