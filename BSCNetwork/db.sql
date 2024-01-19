CREATE TABLE `transactions` (
  `id` int NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `currency_id` int NOT NULL,
  `exchange_rate` decimal(16,4) DEFAULT '0.0000',
  `tx_amount` decimal(19,2) DEFAULT '0.00',
  `before_amount` decimal(19,2) DEFAULT '0.00',
  `after_amount` decimal(19,2) DEFAULT '0.00',
  `tx_type` varchar(35) NOT NULL,
  `change` varchar(15) NOT NULL,
  `tag` varchar(15) DEFAULT NULL,
  `description` tinytext,
  `tran_no` varchar(32) NOT NULL DEFAULT '',
  `remark` varchar(255) DEFAULT '',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `created_by` int DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `currency` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` varchar(25) NOT NULL,
  `priority` int DEFAULT NULL,
  `is_active` tinyint(1) DEFAULT '1',
  `payment_type` varchar(45) DEFAULT NULL,
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_as_currency_code` (`code`),
  UNIQUE KEY `priority_UNIQUE` (`priority`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;


CREATE TABLE `members` (
  `id` int NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `card_number` varchar(10) NOT NULL,
  `card_uuid` varchar(255) DEFAULT NULL,
  `member_name` varchar(255) DEFAULT NULL,
  `name` varchar(255) DEFAULT NULL,
  `grade` varchar(50) DEFAULT NULL,
  `last_grade` varchar(50) DEFAULT NULL,
  `birthdate` date DEFAULT NULL,
  `country_code` varchar(6) DEFAULT NULL,
  `telephone` varchar(100) NOT NULL,
  `email` varchar(150) DEFAULT NULL,
  `gender` varchar(6) DEFAULT NULL,
  `other_contact` varchar(255) DEFAULT NULL,
  `member_category` varchar(155) DEFAULT NULL,
  `status` int NOT NULL DEFAULT '0',
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `remark` varchar(100) DEFAULT NULL,
  `avatar_profile` varchar(255) DEFAULT NULL,
  `fund_password` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `is_acc_deleted` int NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `card_number_UNIQUE` (`card_number`),
  UNIQUE KEY `card_uuid_UNIQUE` (`card_uuid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `wallet` (
  `id` int NOT NULL AUTO_INCREMENT,
  `address` varchar(42) NOT NULL,
  `index` bigint NOT NULL,
  `network` varchar(20) NOT NULL,
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `UC_wallet` (`index`,`network`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `block_transaction` (
  `id` int NOT NULL AUTO_INCREMENT,
  `type` varchar(15) NOT NULL,
  `block_num` bigint NOT NULL,
  `txn` varchar(66) NOT NULL,
  `gas_cost` decimal(15,6) NOT NULL,
  `wallet_index` bigint DEFAULT NULL,
  `sender` varchar(42) NOT NULL,
  `recipient` varchar(42) NOT NULL,
  `amount` decimal(15,6) NOT NULL,
  `token` varchar(10) NOT NULL,
  `is_reconcile` tinyint(1) DEFAULT '0',
  `is_process` tinyint(1) DEFAULT '0',
  `is_gas_transfer` tinyint(1) DEFAULT '0',
  `tran_no` varchar(32) NOT NULL DEFAULT '',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB;

CREATE TABLE `wallet_config` (
  `id` int NOT NULL AUTO_INCREMENT,
  `network` varchar(10) NOT NULL,
  `token` varchar(25) NOT NULL,
  `fee` decimal(15,6) NOT NULL,
  `min_withdraw` decimal(15,6) NOT NULL,
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deposit_status` int NOT NULL DEFAULT '1',
  `withdraw_status` int NOT NULL DEFAULT '1',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB;

CREATE TABLE `user` (
  `id` int NOT NULL AUTO_INCREMENT,
  `username` varchar(50) DEFAULT '',
  `role` varchar(30) DEFAULT '',
  `password` varchar(100) DEFAULT NULL,
  `user_type` varchar(30) DEFAULT NULL,
  `status` int DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `role_id` int DEFAULT NULL,
  `last_login_time` datetime DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  KEY `user_type` (`user_type`) USING BTREE,
  KEY `username` (`username`) USING BTREE,
  KEY `status` (`status`) USING BTREE
) ENGINE=InnoDB;

CREATE TABLE `member_request` (
  `id` int NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `type` varchar(25) NOT NULL,
  `tran_no` varchar(32) NOT NULL DEFAULT '',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB;

CREATE TABLE `member_request_details` (
  `id` int NOT NULL AUTO_INCREMENT,
  `as_member_vip_request_id` int NOT NULL,
  `amount` decimal(15,6) NOT NULL,
  `sender` varchar(45) DEFAULT NULL,
  `recipient` varchar(42) NOT NULL,
  `fee` decimal(15,2) NOT NULL,
  `txn` varchar(66) DEFAULT NULL,
  `token` varchar(25) NOT NULL,
  `tran_no` varchar(32) NOT NULL DEFAULT '',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB;


CREATE TABLE `constant` (
  `id` int NOT NULL AUTO_INCREMENT,
  `value` varchar(255) NOT NULL,
  `label` varchar(255) DEFAULT NULL,
  `category` varchar(255) DEFAULT NULL,
  `tag` varchar(45) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB;