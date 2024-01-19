package cron

import (
	"bsc_network/internal/config"
	"bsc_network/internal/config/log"
	"bsc_network/internal/entity"
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type (
	TXProcessor interface {
		Run(ctx context.Context)
	}

	txProcessor struct {
		logger log.Logger

		listTxStmt                     *sqlx.Stmt
		getMemberLatestAssetStmt       *sqlx.Stmt
		getCurrencyIDStmt              *sqlx.Stmt
		createAssetStmt                *sqlx.NamedStmt
		updateProcessStatusStmt        *sqlx.Stmt
		memberVipStmt                  *sqlx.Stmt
		memberVipQueryByCardNumberStmt *sqlx.Stmt
		constantQueryStmt              *sqlx.Stmt
	}
)

const (
	listTxQuery                string = "SELECT * FROM block_transaction WHERE is_process = 0 AND wallet_index <> 0 AND type = 'DEPOSIT'"
	getMemberLastestAssetQuery string = "SELECT * FROM transactions WHERE user_id = ? AND currency_id = 1 ORDER BY id DESC LIMIT 1"
	getCurrencyIDQuery         string = "SELECT id FROM currency WHERE code = ?"
	createAssetQuery           string = `INSERT INTO 
	transactions(user_id, currency_id, tx_amount, before_amount, after_amount, tx_type, ` + "`change`" + `, description, tran_no) 
  VALUES(:user_id, :currency_id, :tx_amount, :before_amount, :after_amount, :tx_type, :change, :description, :tran_no)`
	updateProcessStatusQuery string = `UPDATE block_transaction SET is_process = 1 WHERE id = ?`
	memberVipQuery           string = `SELECT id, user_id, card_number, member_name FROM members 
  WHERE user_id = ?`
	memberVipByMemberCardQuery string = `SELECT id, user_id, card_number, member_name FROM members 
  WHERE card_number = ?`
	constantQueryQuery string = `SELECT value, label, category, tag FROM constant WHERE category = 'COMPANY_WITHDRAW_ACC'`

	txProcessorInterval time.Duration = 10 * time.Second
)

func NewTxProcessor(ctx context.Context, db *sqlx.DB, logger log.Logger) (TXProcessor, error) {
	t := &txProcessor{
		logger: logger,
	}

	listTxStmt, err := db.PreparexContext(ctx, listTxQuery)
	if err != nil {
		return &txProcessor{}, err
	}

	getMemberLatestAssetStmt, err := db.PreparexContext(ctx, getMemberLastestAssetQuery)
	if err != nil {
		return &txProcessor{}, err
	}

	getCurrencyIDStmt, err := db.PreparexContext(ctx, getCurrencyIDQuery)
	if err != nil {
		return &txProcessor{}, err
	}

	createAssetStmt, err := db.PrepareNamedContext(ctx, createAssetQuery)
	if err != nil {
		return &txProcessor{}, err
	}

	updateProcessStatusStmt, err := db.PreparexContext(ctx, updateProcessStatusQuery)
	if err != nil {
		return &txProcessor{}, err
	}

	memberVipQueryStmt, err := db.PreparexContext(ctx, memberVipQuery)
	if err != nil {
		return &txProcessor{}, err
	}

	memberVipQueryByCardNumberStmt, err := db.PreparexContext(ctx, memberVipByMemberCardQuery)
	if err != nil {
		return &txProcessor{}, err
	}

	constantQueryStmt, err := db.PreparexContext(ctx, constantQueryQuery)
	if err != nil {
		return &txProcessor{}, err
	}

	t.listTxStmt = listTxStmt
	t.getMemberLatestAssetStmt = getMemberLatestAssetStmt
	t.getCurrencyIDStmt = getCurrencyIDStmt
	t.createAssetStmt = createAssetStmt
	t.updateProcessStatusStmt = updateProcessStatusStmt
	t.memberVipStmt = memberVipQueryStmt
	t.memberVipQueryByCardNumberStmt = memberVipQueryByCardNumberStmt
	t.constantQueryStmt = constantQueryStmt

	return t, nil
}

func (t *txProcessor) Run(ctx context.Context) {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				t.logger.Error("txProcessor panic", zap.Stack("stack"))
			}
		}()
		fmt.Printf("\nbefore loop Checking unprocess transaction...\n")

		for {
			t.logger.Info("Checking unprocess transaction...")

			txs, err := t.listTransaction(ctx)
			if err != nil {
				fmt.Printf("\nerror t.listTransaction(ctx)\n")
				t.logger.Errorf("[Run] internal error: %w", err)
				time.Sleep(txProcessorInterval)
				continue
			}

			for _, tx := range txs {
				t.logger.Info("lastAsset entity.ASAsset...")
				var lastAsset entity.ASAsset
				var cashierLastAsset entity.ASAsset

				var cashierVipCardNumber entity.TsConstant
				if err := t.constantQueryStmt.GetContext(ctx, &cashierVipCardNumber); err != nil {
					t.logger.Errorf("[memberVipDetail] get member vip detail error: %s", err)
				}
				fmt.Printf("cashierVipCardNumber %+v\n", cashierVipCardNumber)

				if len(cashierVipCardNumber.Value) < 0 {
					t.logger.Errorf("\nplease setup the cashier account\n")
					os.Exit(1)
				}

				var cashierAccountDetail entity.MemberVip
				if err := t.memberVipQueryByCardNumberStmt.GetContext(ctx, &cashierAccountDetail, cashierVipCardNumber.Value); err != nil {
					t.logger.Errorf("[cashier account] detail not found: %s", err)
					os.Exit(1)
				}

				cashierLastAsset, err = t.getMemberLatestAsset(ctx, cashierAccountDetail.UserId)
				if err != nil {
					t.logger.Errorf("[Run] internal error: %s", err)
					continue
				}

				if cashierLastAsset.AfterAmount.LessThan(tx.Amount) {
					t.logger.Info("[cashierLastAsset] balance is not enough not found: %s", err)
					text := t.cashierBalanceNotEnoughMessage(tx.Amount, tx.Token)
					err := t.sendTelegramBotMessage(text)
					if err != nil {
						continue
					}
					continue
				}

				lastAsset, err = t.getMemberLatestAsset(ctx, tx.WalletIndex)
				if err != nil {
					t.logger.Errorf("[Run] internal error: %s", err)
					continue
				}

				// update cashier asset
				if err = t.createAsset(ctx, cashierLastAsset, tx.Token, tx.Amount, tx.TranNo, entity.Decrease); err != nil {
					t.logger.Errorf("[Run] internal error: %s", err)
				}

				fmt.Printf("update cashier asset")

				// update user asset
				if err = t.createAsset(ctx, lastAsset, tx.Token, tx.Amount, tx.TranNo, entity.Increase); err != nil {
					t.logger.Errorf("[Run] internal error: %s", err)
				}

				fmt.Printf("update user asset")

				if err = t.updateProcessStatus(ctx, tx.ID); err != nil {
					t.logger.Errorf("[Run] internal error: %s", err)
				}

				if err = t.createPushNoti(ctx, tx.WalletIndex, tx.Amount, tx.Token); err != nil {
					t.logger.Errorf("[createPushNoti] internal error: %s", err)
				}
			}
			t.logger.Info("Before... time.Sleep")
			time.Sleep(txProcessorInterval)
		}
	}()
}

func (t *txProcessor) listTransaction(ctx context.Context) ([]entity.CSBlockTransaction, error) {
	var txs []entity.CSBlockTransaction
	if err := t.listTxStmt.SelectContext(ctx, &txs); err != nil {
		return []entity.CSBlockTransaction{}, fmt.Errorf("[listTransaction] internal error: %w", err)
	}

	return txs, nil
}

func (t *txProcessor) getMemberLatestAsset(ctx context.Context, userID int64) (entity.ASAsset, error) {
	var asset entity.ASAsset
	if err := t.getMemberLatestAssetStmt.GetContext(ctx, &asset, userID); err != nil {
		return entity.ASAsset{}, fmt.Errorf("[getMemberLatestAsset] internal error: %w", err)
	}

	return asset, nil
}

func (t *txProcessor) createAsset(
	ctx context.Context,
	lastAsset entity.ASAsset,
	token string,
	amount decimal.Decimal,
	tranNo string,
	txType entity.Change,
) error {
	description := fmt.Sprintf("用户存入 %s %s", amount.StringFixedBank(2), token)
	nullDescription := sql.NullString{
		String: description,
		Valid:  true,
	}
	afterAmount := lastAsset.AfterAmount
	if txType == entity.Increase {
		afterAmount = afterAmount.Add(amount)
	} else {
		afterAmount = afterAmount.Sub(amount)
	}
	newAsset := entity.ASAsset{
		UserID:       lastAsset.UserID,
		CurrencyID:   1,
		TxAmount:     amount,
		BeforeAmount: lastAsset.AfterAmount,
		AfterAmount:  afterAmount,
		TxType:       entity.CryptoDeposit,
		Change:       txType,
		Description:  nullDescription,
		TranNo:       tranNo,
	}
	if _, err := t.createAssetStmt.ExecContext(ctx, &newAsset); err != nil {
		return fmt.Errorf("[updateAsset] internal error: %w", err)
	}

	return nil
}

func (t *txProcessor) sendTelegramBotMessage(text string) error {
	bot, err := tgbotapi.NewBotAPI(config.GlobalConfig.TelegramBot.BotToken)
	if err != nil {
		t.logger.Error("error loading bot", err)
	}
	chatId := config.GlobalConfig.TelegramBot.ChannelId
	message := tgbotapi.NewMessage(chatId, text)

	_, err = bot.Send(message)
	if err != nil {
		t.logger.Error("Error sending bot message", err)
		return err
	}
	return nil
}

func (t *txProcessor) cashierBalanceNotEnoughMessage(amount decimal.Decimal, token string) string {
	text := "现在客户已充值：" + amount.String() + token + "\n" +
		"注：目前出纳账户资金不足。 请先充值，以便用户成功充值至钱包。"
	return text
}

func (t *txProcessor) cryptoDepositTextMessage(memberName string, cardNumber string, amount decimal.Decimal, token string) string {
	text := "亲爱的同事，我们谨通知您，我们收到了新的加密货币存款。 加密货币存款详情如下：\n\n" +
		"客户名称: " + memberName + "\n" +
		"会员账号：" + cardNumber + "\n" +
		"金额数目：" + amount.String() + "\n" +
		"加密货币：" + token
	return text
}

func (t *txProcessor) createPushNoti(ctx context.Context, userId int64, amount decimal.Decimal, token string) error {
	// push notification
	var memberVipDetail entity.MemberVip
	if err := t.memberVipStmt.GetContext(ctx, &memberVipDetail, userId); err != nil {
		return fmt.Errorf("[memberVipDetail] get member vip detail error: %w", err)
	}

	text := t.cryptoDepositTextMessage(memberVipDetail.MemberName.String, memberVipDetail.CardNumber, amount, token)

	err := t.sendTelegramBotMessage(text)
	if err != nil {
		t.logger.Error("[Run] sendTelegramBotMessage internal error: %w", err)
	}

	return nil
}

func (t *txProcessor) updateProcessStatus(ctx context.Context, id int64) error {
	if _, err := t.updateProcessStatusStmt.ExecContext(ctx, id); err != nil {
		return fmt.Errorf("[updateProcessStatus] internal error: %w", err)
	}

	return nil
}
