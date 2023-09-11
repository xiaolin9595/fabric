/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package kvledger

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/ledger/testutil"
	"github.com/hyperledger/fabric/common/util"
	lgr "github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/mock"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/stretchr/testify/require"
)

// TestBackwardCompatibilityRWSetV21 is added to protect against any changes in the hash function
// that is used in preparing the rwset that includes a merkle tree for the range query
func TestBackwardCompatibilityRWSetV21(t *testing.T) {
	rwsetBytes, err := ioutil.ReadFile("testdata/rwsetbytes_v21")
	require.NoError(t, err)
	b := testGenerateSampleRWSet(t)
	require.Equal(t, rwsetBytes, b)
}

// TestGenerateSampleRWSet generates the rwset that includes a merkle tree for a range-query read-set as well
// The data present in the file testdata/rwsetbytes_v21 is generated by running the below test code on version 2.1
// To regenrate the data, (if needed in order to add some more data to the generated rwset), checkout release-2.1,
// and uncomment and run the following test
// func TestGenerateSampleRWSet(t *testing.T) {
// 	b := testGenerateSampleRWSet(t)
// 	require.NoError(t, ioutil.WriteFile("testdata/rwsetbytes_v21", b, 0644))
// }

func testGenerateSampleRWSet(t *testing.T) []byte {
	conf, cleanup := testConfig(t)
	defer cleanup()
	provider := testutilNewProvider(conf, t, &mock.DeployedChaincodeInfoProvider{})
	defer provider.Close()

	bg, gb := testutil.NewBlockGenerator(t, "testLedger", false)
	gbHash := protoutil.BlockHeaderHash(gb.Header)
	ledger, err := provider.CreateFromGenesisBlock(gb)
	require.NoError(t, err)
	defer ledger.Close()

	bcInfo, err := ledger.GetBlockchainInfo()
	require.NoError(t, err)
	require.Equal(t, &common.BlockchainInfo{
		Height: 1, CurrentBlockHash: gbHash, PreviousBlockHash: nil,
	}, bcInfo)

	txid := util.GenerateUUID()

	// perform a range query for significant larger scan so that the merkle tree building kicks in
	// each level contains max 50 nodes per the current configuration
	simulator, err := ledger.NewTxSimulator(txid)
	require.NoError(t, err)
	for i := 0; i < 10011; i++ {
		require.NoError(t, simulator.SetState("ns1", fmt.Sprintf("key-%000d", i), []byte(fmt.Sprintf("value-%000d", i))))
	}
	simulator.Done()
	simRes, err := simulator.GetTxSimulationResults()
	require.NoError(t, err)
	pubSimBytes, err := simRes.GetPubSimulationBytes()
	require.NoError(t, err)
	block1 := bg.NextBlock([][]byte{pubSimBytes})
	require.NoError(t, ledger.CommitLegacy(&lgr.BlockAndPvtData{Block: block1}, &lgr.CommitOptions{}))

	simulator, err = ledger.NewTxSimulator(txid)
	require.NoError(t, err)
	_, err = simulator.GetState("ns1", fmt.Sprintf("key-%000d", 5))
	require.NoError(t, err)
	require.NoError(t, simulator.SetState("ns1", fmt.Sprintf("key-%000d", 6), []byte(fmt.Sprintf("value-%000d-new", 6))))
	itr, err := simulator.GetStateRangeScanIterator("ns1", "", "")
	require.NoError(t, err)
	numKVs := 0
	for {
		kv, err := itr.Next()
		require.NoError(t, err)
		if kv == nil {
			break
		}
		numKVs++
	}
	require.Equal(t, 10011, numKVs)
	simulator.Done()
	simRes, err = simulator.GetTxSimulationResults()
	require.NoError(t, err)
	pubSimBytes, err = simRes.GetPubSimulationBytes()
	require.NoError(t, err)
	return pubSimBytes
}
