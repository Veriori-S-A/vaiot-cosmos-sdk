package middleware_test

import (
	"context"
	"errors"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/x/auth/middleware"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
)

// txTest is a dummy tx that doesn't implement GasTx. It should set the GasMeter
// to 0 in this case.
type txTest struct{}

var _ sdk.Tx = txTest{}

func (t txTest) GetMsgs() []sdk.Msg   { return []sdk.Msg{} }
func (t txTest) ValidateBasic() error { return nil }

func (s *MWTestSuite) setupGasTx() (signing.Tx, []byte, sdk.Context, uint64) {
	ctx := s.SetupTest(true)
	txBuilder := s.clientCtx.TxConfig.NewTxBuilder()

	// keys and addresses
	priv1, _, addr1 := testdata.KeyTestPubAddr()

	// msg and signatures
	msg := testdata.NewTestMsg(addr1)
	feeAmount := testdata.NewTestFeeAmount()
	gasLimit := testdata.NewTestGasLimit()
	s.Require().NoError(txBuilder.SetMsgs(msg))
	txBuilder.SetFeeAmount(feeAmount)
	txBuilder.SetGasLimit(gasLimit)

	// test tx
	privs, accNums, accSeqs := []cryptotypes.PrivKey{priv1}, []uint64{0}, []uint64{0}
	tx, txBytes, err := s.createTestTx(txBuilder, privs, accNums, accSeqs, ctx.ChainID())
	s.Require().NoError(err)

	// Set height to non-zero value for GasMeter to be set
	ctx = ctx.WithBlockHeight(1)

	return tx, txBytes, ctx, gasLimit
}

func (s *MWTestSuite) TestSetup() {
	testTx, _, ctx, gasLimit := s.setupGasTx()
	txHandler := middleware.ComposeMiddlewares(noopTxHandler{}, middleware.GasTxMiddleware)

	testcases := []struct {
		name        string
		tx          sdk.Tx
		expGasLimit uint64
		expErr      bool
		errorStr    string
	}{
		{"not a gas tx", txTest{}, 0, true, "Tx must be GasTx: tx parse error"},
		{"tx with its own gas limit", testTx, gasLimit, false, ""},
	}
	for _, tc := range testcases {
		s.Run(tc.name, func() {
			res, _, err := txHandler.CheckTx(sdk.WrapSDKContext(ctx), tx.Request{Tx: testTx}, tx.RequestCheckTx{})
			if tc.expErr {
				s.Require().EqualError(err, tc.errorStr)
			} else {
				s.Require().Nil(err, "SetUpContextMiddleware returned error")
				s.Require().Equal(tc.expGasLimit, uint64(res.GasWanted))
			}
		})
	}
}

func (s *MWTestSuite) TestRecoverPanic() {
	testTx, txBytes, ctx, gasLimit := s.setupGasTx()
	txHandler := middleware.ComposeMiddlewares(outOfGasTxHandler{}, middleware.GasTxMiddleware, middleware.RecoveryTxMiddleware)
	res, _, err := txHandler.CheckTx(sdk.WrapSDKContext(ctx), tx.Request{Tx: testTx, TxBytes: txBytes}, tx.RequestCheckTx{})
	s.Require().Error(err, "Did not return error on OutOfGas panic")
	s.Require().True(errors.Is(sdkerrors.ErrOutOfGas, err), "Returned error is not an out of gas error")
	s.Require().Equal(gasLimit, uint64(res.GasWanted))

	txHandler = middleware.ComposeMiddlewares(outOfGasTxHandler{}, middleware.GasTxMiddleware)
	s.Require().Panics(func() {
		txHandler.CheckTx(sdk.WrapSDKContext(ctx), tx.Request{Tx: testTx, TxBytes: txBytes}, tx.RequestCheckTx{})
	}, "Recovered from non-Out-of-Gas panic")
}

// outOfGasTxHandler is a test middleware that will throw OutOfGas panic.
type outOfGasTxHandler struct{}

var _ tx.Handler = outOfGasTxHandler{}

func (txh outOfGasTxHandler) DeliverTx(ctx context.Context, _ tx.Request) (tx.Response, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	overLimit := sdkCtx.GasMeter().Limit() + 1

	// Should panic with outofgas error
	sdkCtx.GasMeter().ConsumeGas(overLimit, "test panic")

	panic("not reached")
}
func (txh outOfGasTxHandler) CheckTx(ctx context.Context, _ tx.Request, _ tx.RequestCheckTx) (tx.Response, tx.ResponseCheckTx, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	overLimit := sdkCtx.GasMeter().Limit() + 1

	// Should panic with outofgas error
	sdkCtx.GasMeter().ConsumeGas(overLimit, "test panic")

	panic("not reached")
}
func (txh outOfGasTxHandler) SimulateTx(ctx context.Context, _ tx.Request) (tx.Response, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	overLimit := sdkCtx.GasMeter().Limit() + 1

	// Should panic with outofgas error
	sdkCtx.GasMeter().ConsumeGas(overLimit, "test panic")

	panic("not reached")
}

// noopTxHandler is a test middleware that does nothing.
type noopTxHandler struct{}

var _ tx.Handler = noopTxHandler{}

func (txh noopTxHandler) CheckTx(_ context.Context, _ tx.Request, _ tx.RequestCheckTx) (tx.Response, tx.ResponseCheckTx, error) {
	return tx.Response{}, tx.ResponseCheckTx{}, nil
}
func (txh noopTxHandler) SimulateTx(_ context.Context, _ tx.Request) (tx.Response, error) {
	return tx.Response{}, nil
}
func (txh noopTxHandler) DeliverTx(ctx context.Context, _ tx.Request) (tx.Response, error) {
	return tx.Response{}, nil
}
