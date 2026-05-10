package legalentity

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "crm-distributed/proto/document"
	"crm-distributed/shared/domain"
)

type GRPCHandler struct {
	pb.UnimplementedDocumentServiceServer
	uc  *Usecase
	log *slog.Logger
}

func NewGRPCHandler(uc *Usecase, log *slog.Logger) *GRPCHandler {
	return &GRPCHandler{uc: uc, log: log}
}

func (h *GRPCHandler) CreateLegalEntity(ctx context.Context, req *pb.CreateLegalEntityRequest) (*pb.CreateLegalEntityResponse, error) {
	companyUUID, err := uuid.Parse(req.CompanyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid company_id")
	}

	cmd := CreateEntityCommand{
		CompanyUUID:   companyUUID,
		Name:          req.Name,
		INN:           req.Inn,
		KPP:           req.Kpp,
		LegalAddress:  req.Address,
		ActualAddress: req.Address,
	}

	le, err := h.uc.CreateEntity(ctx, cmd)
	if err != nil {
		return nil, h.mapError(ctx, err, "create legal entity")
	}

	return &pb.CreateLegalEntityResponse{
		Id:        le.UUID.String(),
		CreatedAt: le.CreatedAt.Unix(),
	}, nil
}

func (h *GRPCHandler) GetLegalEntity(ctx context.Context, req *pb.GetLegalEntityRequest) (*pb.GetLegalEntityResponse, error) {
	uid, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	le, err := h.uc.GetEntity(ctx, uid)
	if err != nil {
		return nil, h.mapError(ctx, err, "get legal entity")
	}

	return &pb.GetLegalEntityResponse{
		Entity: toLegalEntityProto(le),
	}, nil
}

func (h *GRPCHandler) CreateBankAccount(ctx context.Context, req *pb.CreateBankAccountRequest) (*pb.CreateBankAccountResponse, error) {
	legalEntityUUID, err := uuid.Parse(req.LegalEntityId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid legal_entity_id")
	}

	cmd := CreateAccountCommand{
		LegalEntityUUID: legalEntityUUID,
		Bank:            req.BankName,
		BIK:             req.Bik,
		PayAcc:          req.AccountNumber,
		CorrAcc:         req.CorrAccount,
		Currency:        "RUB",
	}

	ba, err := h.uc.CreateAccount(ctx, cmd)
	if err != nil {
		return nil, h.mapError(ctx, err, "create bank account")
	}

	return &pb.CreateBankAccountResponse{
		Id:        ba.UUID.String(),
		CreatedAt: ba.CreatedAt.Unix(),
	}, nil
}

func (h *GRPCHandler) ListBankAccounts(ctx context.Context, req *pb.ListBankAccountsRequest) (*pb.ListBankAccountsResponse, error) {
	legalEntityUUID, err := uuid.Parse(req.LegalEntityId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid legal_entity_id")
	}

	accounts, err := h.uc.ListAccounts(ctx, legalEntityUUID)
	if err != nil {
		return nil, h.mapError(ctx, err, "list bank accounts")
	}

	pbAccounts := make([]*pb.BankAccount, 0, len(accounts))
	for i := range accounts {
		pbAccounts = append(pbAccounts, toBankAccountProto(&accounts[i]))
	}

	return &pb.ListBankAccountsResponse{Accounts: pbAccounts}, nil
}

func toLegalEntityProto(le *domain.LegalEntity) *pb.LegalEntity {
	return &pb.LegalEntity{
		Id:        le.UUID.String(),
		CompanyId: le.CompanyUUID.String(),
		Name:      le.Name,
		Inn:       le.INN,
		Kpp:       le.KPP,
		Ogrn:      "",
		Address:   le.LegalAddress,
		CreatedAt: le.CreatedAt.Unix(),
	}
}

func toBankAccountProto(ba *domain.BankAccount) *pb.BankAccount {
	return &pb.BankAccount{
		Id:            ba.UUID.String(),
		LegalEntityId: ba.LegalEntityUUID.String(),
		BankName:      ba.Bank,
		Bik:           ba.BIK,
		AccountNumber: ba.PayAcc,
		CorrAccount:   ba.CorrAcc,
		CreatedAt:     ba.CreatedAt.Unix(),
	}
}

func (h *GRPCHandler) mapError(ctx context.Context, err error, op string) error {
	switch {
	case errors.Is(err, domain.ErrLegalEntityNotFound),
		errors.Is(err, domain.ErrBankAccountNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, domain.ErrLegalEntityAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())

	case errors.Is(err, domain.ErrLegalEntityInvalid):
		return status.Error(codes.InvalidArgument, err.Error())

	default:
		h.log.ErrorContext(ctx, "grpc handler error", "op", op, "error", err)
		return status.Error(codes.Internal, "internal error")
	}
}

func LoggingInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		level := slog.LevelInfo
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == codes.Internal || st.Code() == codes.Unknown {
				level = slog.LevelError
			} else {
				level = slog.LevelWarn
			}
		}

		log.Log(ctx, level, "grpc call",
			"method", info.FullMethod,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err,
		)

		return resp, err
	}
}
