package ethereumlisten

import (
	"context"
	"fmt"
	"github.com/astaxie/beego/logs"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"poly-bridge/basedef"
	"poly-bridge/go_abi/eccm_abi"
	nftlp "poly-bridge/go_abi/nft_lock_proxy_abi"
	nftwp "poly-bridge/go_abi/nft_wrap_abi"
	"poly-bridge/models"
)

func (e *EthereumChainListen) NFTWrapperAddress() common.Address {
	return common.HexToAddress(e.ethCfg.NFTWrapperContract)
}

func (e *EthereumChainListen) ECCMAddress() common.Address {
	return common.HexToAddress(e.ethCfg.CCMContract)
}

func (e *EthereumChainListen) NFTProxyAddress() common.Address {
	return common.HexToAddress(e.ethCfg.NFTProxyContract)
}

func (e *EthereumChainListen) HandleNFTNewBlock(
	height uint64, tt uint64,
	eccmLockEvents []*models.ECCMLockEvent,
	eccmUnLockEvents []*models.ECCMUnlockEvent) (
	[]*models.WrapperTransaction,
	[]*models.SrcTransaction,
	[]*models.PolyTransaction,
	[]*models.DstTransaction,
	error,
) {

	wrapAddr := e.NFTWrapperAddress()
	//eccmAddr := e.ECCMAddress()
	proxyAddr := e.NFTProxyAddress()
	chainName := e.GetChainName()
	chainID := e.GetChainId()

	//blockHeader, err := e.ethSdk.GetHeaderByNumber(height)
	//if err != nil {
	//	return nil, nil, nil, nil, err
	//}
	//if blockHeader == nil {
	//	return nil, nil, nil, nil, fmt.Errorf("there is no ethereum block!")
	//}
	//tt := blockHeader.Time

	wrapperTransactions, err := e.getNFTWrapperEventByBlockNumber(wrapAddr, height, height)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for _, wtx := range wrapperTransactions {
		logs.Info("(wrapper) from chain: %s, txhash: %s", chainName, wtx.Hash)
		wtx.Time = tt
		wtx.SrcChainId = e.GetChainId()
		wtx.Status = basedef.STATE_SOURCE_DONE
	}
	//eccmLockEvents, eccmUnLockEvents, err := e.getNFTECCMEventByBlockNumber(eccmAddr, height, height)
	//if err != nil {
	//	return nil, nil, nil, nil, err
	//}
	proxyLockEvents, proxyUnlockEvents, err := e.getNFTProxyEventByBlockNumber(proxyAddr, height, height)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	srcTransactions := make([]*models.SrcTransaction, 0)
	dstTransactions := make([]*models.DstTransaction, 0)
	for _, lockEvent := range eccmLockEvents {
		if lockEvent.Method == _eth_crosschainlock {
			logs.Info("(lock) from chain: %s, txhash: %s, txid: %s",
				chainName, lockEvent.TxHash, lockEvent.Txid)
			srcTransaction := assembleSrcTransaction(lockEvent, proxyLockEvents, chainID, tt)
			srcTransactions = append(srcTransactions, srcTransaction)
		}
	}
	// save unLockEvent to db
	for _, unLockEvent := range eccmUnLockEvents {
		if unLockEvent.Method == _eth_crosschainunlock {
			logs.Info("(unlock) to chain: %s, txhash: %s", chainName, unLockEvent.TxHash)
			dstTransaction := assembleDstTransaction(unLockEvent, proxyUnlockEvents, chainID, tt)
			dstTransactions = append(dstTransactions, dstTransaction)
		}
	}
	return wrapperTransactions, srcTransactions, nil, dstTransactions, nil
}

func (e *EthereumChainListen) HandleNFTBlockBatch(
	startHeight, endHeight uint64,
	eccmLockEvents []*models.ECCMLockEvent,
	eccmUnLockEvents []*models.ECCMUnlockEvent,
) (
	[]*models.WrapperTransaction,
	[]*models.SrcTransaction,
	[]*models.PolyTransaction,
	[]*models.DstTransaction,
	error,
) {

	wrapAddr := e.NFTWrapperAddress()
	//eccmAddr := e.ECCMAddress()
	proxyAddr := e.NFTProxyAddress()
	chainName := e.GetChainName()
	chainID := e.GetChainId()

	wrapperTransactions, err := e.getNFTWrapperEventByBlockNumber(wrapAddr, startHeight, endHeight)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for _, wtx := range wrapperTransactions {
		logs.Info("(wrapper) from chain: %s, txhash: %s", chainName, wtx.Hash)
		wtx.SrcChainId = e.GetChainId()
		wtx.Status = basedef.STATE_SOURCE_DONE
	}
	//eccmLockEvents, eccmUnLockEvents, err := e.getNFTECCMEventByBlockNumber(eccmAddr, startHeight, endHeight)
	//if err != nil {
	//	return nil, nil, nil, nil, err
	//}
	proxyLockEvents, proxyUnlockEvents, err := e.getNFTProxyEventByBlockNumber(proxyAddr, startHeight, endHeight)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	//

	srcTransactions := make([]*models.SrcTransaction, 0)
	dstTransactions := make([]*models.DstTransaction, 0)
	for _, lockEvent := range eccmLockEvents {
		if lockEvent.Method == _eth_crosschainlock {
			logs.Info("(lock) from chain: %s, txhash: %s, txid: %s", chainName, lockEvent.TxHash, lockEvent.Txid)
			srcTransaction := assembleSrcTransaction(lockEvent, proxyLockEvents, chainID, 0)
			srcTransactions = append(srcTransactions, srcTransaction)
		}
	}
	// save unLockEvent to db
	for _, unLockEvent := range eccmUnLockEvents {
		if unLockEvent.Method == _eth_crosschainunlock {
			logs.Info("(unlock) to chain: %s, txhash: %s", chainName, unLockEvent.TxHash)
			dstTransaction := assembleDstTransaction(unLockEvent, proxyUnlockEvents, chainID, 0)
			dstTransactions = append(dstTransactions, dstTransaction)
		}
	}
	return wrapperTransactions, srcTransactions, nil, dstTransactions, nil
}

func (e *EthereumChainListen) getNFTWrapperEventByBlockNumber(
	wrapAddr common.Address,
	startHeight, endHeight uint64) (
	[]*models.WrapperTransaction,
	error,
) {

	// todo: newPolyWrapper change to IPolyNFTWrapper
	wrapperContract, err := nftwp.NewPolyNFTWrapper(wrapAddr, e.ethSdk.GetClient())
	if err != nil {
		return nil, fmt.Errorf("GetSmartContractEventByBlock, error: %s", err.Error())
	}
	opt := &bind.FilterOpts{
		Start:   startHeight,
		End:     &endHeight,
		Context: context.Background(),
	}

	// get ethereum lock events from given block
	wrapperTransactions := make([]*models.WrapperTransaction, 0)
	lockEvents, err := wrapperContract.FilterPolyWrapperLock(opt, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("GetSmartContractEventByBlock, filter lock events :%s", err.Error())
	}
	for lockEvents.Next() {
		evt := lockEvents.Event
		wtx := wrapLockEvent2WrapTx(evt)
		wrapperTransactions = append(wrapperTransactions, wtx)
	}
	speedupEvents, err := wrapperContract.FilterPolyWrapperSpeedUp(opt, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("GetSmartContractEventByBlock, filter lock events :%s", err.Error())
	}
	for speedupEvents.Next() {
		evt := speedupEvents.Event
		wtx := wrapSpeedUpEvent2WrapTx(evt)
		wrapperTransactions = append(wrapperTransactions, wtx)
	}
	return wrapperTransactions, nil
}

func (e *EthereumChainListen) getNFTECCMEventByBlockNumber(
	eccmAddr common.Address,
	startHeight, endHeight uint64) (
	[]*models.ECCMLockEvent,
	[]*models.ECCMUnlockEvent,
	error,
) {

	eccmContract, err := eccm_abi.NewEthCrossChainManager(eccmAddr, e.ethSdk.GetClient())
	if err != nil {
		return nil, nil, fmt.Errorf("GetSmartContractEventByBlock, error: %s", err.Error())
	}
	opt := &bind.FilterOpts{
		Start:   startHeight,
		End:     &endHeight,
		Context: context.Background(),
	}
	// get ethereum lock events from given block
	eccmLockEvents := make([]*models.ECCMLockEvent, 0)
	crossChainEvents, err := eccmContract.FilterCrossChainEvent(opt, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("GetSmartContractEventByBlock, filter lock events :%s", err.Error())
	}
	for crossChainEvents.Next() {
		evt := crossChainEvents.Event
		Fee := e.GetConsumeGas(evt.Raw.TxHash)
		eccmLockEvent := crossChainEvent2ProxyLockEvent(evt, Fee)
		eccmLockEvents = append(eccmLockEvents, eccmLockEvent)
	}
	// ethereum unlock events from given block
	eccmUnlockEvents := make([]*models.ECCMUnlockEvent, 0)
	executeTxEvent, err := eccmContract.FilterVerifyHeaderAndExecuteTxEvent(opt)
	if err != nil {
		return nil, nil, fmt.Errorf("GetSmartContractEventByBlock, filter unlock events :%s", err.Error())
	}

	for executeTxEvent.Next() {
		evt := executeTxEvent.Event
		Fee := e.GetConsumeGas(evt.Raw.TxHash)
		eccmUnlockEvent := verifyAndExecuteEvent2ProxyUnlockEvent(evt, Fee)
		eccmUnlockEvents = append(eccmUnlockEvents, eccmUnlockEvent)
	}
	return eccmLockEvents, eccmUnlockEvents, nil
}

func (e *EthereumChainListen) getNFTProxyEventByBlockNumber(
	proxyAddr common.Address,
	startHeight, endHeight uint64) (
	[]*models.ProxyLockEvent,
	[]*models.ProxyUnlockEvent,
	error,
) {

	proxyContract, err := nftlp.NewPolyNFTLockProxy(proxyAddr, e.ethSdk.GetClient())
	if err != nil {
		return nil, nil, fmt.Errorf("GetSmartContractEventByBlock, error: %s", err.Error())
	}
	opt := &bind.FilterOpts{
		Start:   startHeight,
		End:     &endHeight,
		Context: context.Background(),
	}
	// get ethereum lock events from given block
	proxyLockEvents := make([]*models.ProxyLockEvent, 0)
	lockEvents, err := proxyContract.FilterLockEvent(opt)
	if err != nil {
		return nil, nil, fmt.Errorf("GetSmartContractEventByBlock, filter lock events :%s", err.Error())
	}
	for lockEvents.Next() {
		proxyLockEvent := convertLockProxyEvent(lockEvents.Event)
		proxyLockEvents = append(proxyLockEvents, proxyLockEvent)
	}

	// ethereum unlock events from given block
	proxyUnlockEvents := make([]*models.ProxyUnlockEvent, 0)
	unlockEvents, err := proxyContract.FilterUnlockEvent(opt)
	if err != nil {
		return nil, nil, fmt.Errorf("GetSmartContractEventByBlock, filter unlock events :%s", err.Error())
	}
	for unlockEvents.Next() {
		proxyUnlockEvent := convertUnlockProxyEvent(unlockEvents.Event)
		proxyUnlockEvents = append(proxyUnlockEvents, proxyUnlockEvent)
	}
	return proxyLockEvents, proxyUnlockEvents, nil
}