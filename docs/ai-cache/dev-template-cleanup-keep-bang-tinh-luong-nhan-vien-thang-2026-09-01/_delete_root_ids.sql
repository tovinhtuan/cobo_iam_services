SET NAMES utf8mb4;
SELECT dt.type_id
FROM disclosure_types dt
WHERE dt.type_id COLLATE utf8mb4_unicode_ci <> 'bang-tinh-luong-nhan-vien-ban-sao-2' COLLATE utf8mb4_unicode_ci
ORDER BY dt.type_id;
