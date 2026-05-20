SET NAMES utf8mb4;

DELETE FROM disclosure_type_versions
WHERE type_id IN (
  'dt-obligation-report',
  'dt-shareholder-meeting',
  'dt-disclosure-transaction'
);

DELETE FROM disclosure_types
WHERE type_id IN (
  'dt-obligation-report',
  'dt-shareholder-meeting',
  'dt-disclosure-transaction'
);
