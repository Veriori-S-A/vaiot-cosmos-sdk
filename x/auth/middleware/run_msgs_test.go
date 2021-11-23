package middleware_test

import (
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/x/auth/middleware"
)

func (s *MWTestSuite) TestRunMsgs() {
	ctx := s.SetupTest(true) // setup

	msr := middleware.NewMsgServiceRouter(s.clientCtx.InterfaceRegistry)
	testdata.RegisterMsgServer(msr, testdata.MsgServerImpl{})
	txHandler := middleware.NewRunMsgsTxHandler(msr, nil)

	priv, _, _ := testdata.KeyTestPubAddr()
	txBuilder := s.clientCtx.TxConfig.NewTxBuilder()
	txBuilder.SetMsgs(&testdata.MsgCreateDog{Dog: &testdata.Dog{Name: "Spot"}})
	privs, accNums, accSeqs := []cryptotypes.PrivKey{priv}, []uint64{0}, []uint64{0}
	testTx, _, err := s.createTestTx(txBuilder, privs, accNums, accSeqs, ctx.ChainID())
	s.Require().NoError(err)
	txBytes, err := s.clientCtx.TxConfig.TxEncoder()(testTx)
	s.Require().NoError(err)

	res, err := txHandler.DeliverTx(sdk.WrapSDKContext(ctx), tx.Request{Tx: testTx, TxBytes: txBytes})
	s.Require().NoError(err)
	s.Require().NotEmpty(res.MsgResponses)
	var txMsgData sdk.TxMsgData
	err = s.clientCtx.Codec.Unmarshal(res.Data, &txMsgData)
	s.Require().NoError(err)
	s.Require().Len(txMsgData.Data, 1)
	s.Require().Equal(sdk.MsgTypeURL(&testdata.MsgCreateDog{}), txMsgData.Data[0].MsgType)
}
