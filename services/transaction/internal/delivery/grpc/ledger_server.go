package grpcdelivery

import (
	"context"

	"github.com/example/fintech-core-api/services/transaction/internal/usecase"
	"google.golang.org/grpc"
)

const ledgerServiceName = "fintech.ledger.v1.LedgerService"

type GetAccountBalanceRequest struct {
	AccountID string `json:"account_id"`
}

type GetAccountBalanceResponse struct {
	AccountID    string `json:"account_id"`
	OwnerUserID  string `json:"owner_user_id"`
	Currency     string `json:"currency"`
	BalanceCents int64  `json:"balance_cents"`
}

type LedgerServiceServer interface {
	GetAccountBalance(context.Context, *GetAccountBalanceRequest) (*GetAccountBalanceResponse, error)
}

type LedgerServer struct {
	transactions *usecase.TransactionUsecase
}

func NewLedgerServer(transactions *usecase.TransactionUsecase) *LedgerServer {
	return &LedgerServer{transactions: transactions}
}

func (s *LedgerServer) GetAccountBalance(ctx context.Context, req *GetAccountBalanceRequest) (*GetAccountBalanceResponse, error) {
	account, err := s.transactions.GetAccountInternal(ctx, req.AccountID)
	if err != nil {
		return nil, err
	}
	return &GetAccountBalanceResponse{
		AccountID:    account.ID,
		OwnerUserID:  account.OwnerUserID,
		Currency:     account.Currency,
		BalanceCents: account.BalanceCents,
	}, nil
}

func RegisterLedgerService(server *grpc.Server, service LedgerServiceServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: ledgerServiceName,
		HandlerType: (*LedgerServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "GetAccountBalance",
				Handler:    getAccountBalanceHandler,
			},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "proto/ledger/ledger.proto",
	}, service)
}

func getAccountBalanceHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetAccountBalanceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LedgerServiceServer).GetAccountBalance(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/" + ledgerServiceName + "/GetAccountBalance",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LedgerServiceServer).GetAccountBalance(ctx, req.(*GetAccountBalanceRequest))
	}
	return interceptor(ctx, in, info, handler)
}
