package handler

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"crm-distributed/cmd/notification-service/internal/usecase"
	pb "crm-distributed/proto/notification"
)

type GRPCHandler struct {
	pb.UnimplementedNotificationServiceServer
	uc  *usecase.NotificationUsecase
	log *slog.Logger
}

func NewGRPCHandler(uc *usecase.NotificationUsecase, log *slog.Logger) *GRPCHandler {
	return &GRPCHandler{uc: uc, log: log}
}

func (h *GRPCHandler) Send(ctx context.Context, req *pb.SendRequest) (*pb.SendResponse, error) {
	if len(req.UserIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_ids is required")
	}

	notifID, delivered, err := h.uc.Send(
		ctx,
		req.UserIds,
		int32(req.Type),
		req.Title,
		req.Body,
		req.RefId,
	)
	if err != nil {
		h.log.ErrorContext(ctx, "grpc Send failed", "error", err)
		return nil, status.Errorf(codes.Internal, "send notification: %v", err)
	}

	return &pb.SendResponse{
		NotificationId: notifID,
		DeliveredCount: delivered,
	}, nil
}

func (h *GRPCHandler) GetUnread(ctx context.Context, req *pb.GetUnreadRequest) (*pb.GetUnreadResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	notifications, err := h.uc.GetUnread(ctx, req.UserId, int(req.Limit))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get unread: %v", err)
	}

	pbNotifs := make([]*pb.Notification, 0, len(notifications))
	for _, n := range notifications {
		pbNotifs = append(pbNotifs, &pb.Notification{
			Id:        n.ID,
			UserId:    n.UserID,
			Type:      pb.NotificationType(n.Type),
			Title:     n.Title,
			Body:      n.Body,
			RefId:     n.RefID,
			IsRead:    n.IsRead,
			CreatedAt: n.CreatedAt.Unix(),
		})
	}

	return &pb.GetUnreadResponse{Notifications: pbNotifs}, nil
}

func (h *GRPCHandler) MarkRead(ctx context.Context, req *pb.MarkReadRequest) (*pb.MarkReadResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	count, err := h.uc.MarkRead(ctx, req.UserId, req.NotifIds)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mark read: %v", err)
	}

	return &pb.MarkReadResponse{UpdatedCount: count}, nil
}

func LoggingInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		log.InfoContext(ctx, "grpc call",
			"method", info.FullMethod,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err,
		)

		return resp, err
	}
}
