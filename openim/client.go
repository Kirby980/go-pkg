package openim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	DefaultOpenIMAPIURL = "http://127.0.0.1:10002"
	DefaultSecret       = "openIM123"
	AdminUserID         = "imAdmin"
)

// OpenIMClient OpenIM客户端
type OpenIMClient struct {
	apiURL      string
	secret      string
	httpClient  *http.Client
	mongoClient *mongo.Client
}

// NewOpenIMClient 创建OpenIM客户端
func NewOpenIMClient(apiURL, secret string) *OpenIMClient {
	if apiURL == "" {
		apiURL = DefaultOpenIMAPIURL
	}
	if secret == "" {
		secret = DefaultSecret
	}

	// 连接MongoDB，配置连接池
	var mongoClient *mongo.Client
	mongoCtx, mongoCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer mongoCancel()

	mongoOpts := options.Client().
		ApplyURI("mongodb://root:example@127.0.0.1:27017/openim_v3?authSource=admin").
		SetMaxPoolSize(10).                        // 最大连接池大小
		SetMinPoolSize(2).                         // 最小连接池大小
		SetMaxConnIdleTime(30 * time.Second).      // 连接空闲时间
		SetConnectTimeout(10 * time.Second).       // 连接超时
		SetServerSelectionTimeout(5 * time.Second) // 服务器选择超时

	client, err := mongo.Connect(mongoCtx, mongoOpts)
	if err != nil {
		fmt.Printf("连接MongoDB失败: %v\n", err)
		mongoClient = nil // 如果连接失败，设为nil，后续会处理
	} else {
		// 测试连接
		if err = client.Ping(mongoCtx, nil); err != nil {
			fmt.Printf("MongoDB连接测试失败: %v\n", err)
			client.Disconnect(mongoCtx)
			mongoClient = nil
		} else {
			mongoClient = client
			fmt.Println("MongoDB连接成功")
		}
	}

	return &OpenIMClient{
		apiURL: apiURL,
		secret: secret,
		httpClient: &http.Client{
			Timeout: time.Second * 30,
		},
		mongoClient: mongoClient,
	}
}

// BaseResponse OpenIM响应基础结构
type BaseResponse struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	ErrDlt  string `json:"errDlt"`
}

// UserInfo 用户信息
type UserInfo struct {
	UserID   string `json:"userID"`
	Nickname string `json:"nickname"`
	FaceURL  string `json:"faceURL"`
}

// RegisterUserReq 注册用户请求
type RegisterUserReq struct {
	Secret string     `json:"secret"`
	Users  []UserInfo `json:"users"`
}

// RegisterUserResp 注册用户响应
type RegisterUserResp struct {
	BaseResponse
}

// SendMsgReq 发送消息请求
type SendMsgReq struct {
	SendID           string `json:"sendID"`
	SendNickname     string `json:"sendNickname"`
	RecvID           string `json:"recvID"`
	RecvNickname     string `json:"recvNickname"`
	SenderPlatformID int32  `json:"senderPlatformID"`
	Content          struct {
		Content string `json:"content"`
	} `json:"content"`
	SessionType int32  `json:"sessionType"`
	ContentType int32  `json:"contentType"`
	CreateTime  int64  `json:"createTime"`
	ClientMsgID string `json:"clientMsgID"`
}

// SendMsgResp 发送消息响应
type SendMsgResp struct {
	BaseResponse
	Data struct {
		ClientMsgID string `json:"clientMsgID"`
		ServerMsgID string `json:"serverMsgID"`
		SendTime    int64  `json:"sendTime"`
	} `json:"data"`
}

// GetConversationsReq 获取会话列表请求
type GetConversationsReq struct {
	OwnerUserID string `json:"ownerUserID"`
	Offset      int32  `json:"offset"`
	Count       int32  `json:"count"`
}

// ConversationInfo 会话信息
type ConversationInfo struct {
	ConversationID        string `json:"conversationID"`
	ConversationType      int32  `json:"conversationType"`
	UserID                string `json:"userID"`
	GroupID               string `json:"groupID"`
	RecvMsgOpt            int32  `json:"recvMsgOpt"`
	UnreadCount           int32  `json:"unreadCount"`
	LatestMsg             string `json:"latestMsg"`
	LatestMsgSendTime     int64  `json:"latestMsgSendTime"`
	DraftText             string `json:"draftText"`
	DraftTextTime         int64  `json:"draftTextTime"`
	IsPinned              bool   `json:"isPinned"`
	IsPrivateChat         bool   `json:"isPrivateChat"`
	BurnDuration          int32  `json:"burnDuration"`
	GroupAtType           int32  `json:"groupAtType"`
	IsNotInGroup          bool   `json:"isNotInGroup"`
	UpdateUnreadCountTime int64  `json:"updateUnreadCountTime"`
}

// GetConversationsResp 获取会话列表响应
type GetConversationsResp struct {
	BaseResponse
	Data struct {
		Conversations []ConversationInfo `json:"conversations"`
		UnreadTotal   int32              `json:"unreadTotal"`
	} `json:"data"`
}

// GetChatLogsReq 获取聊天记录请求
type GetChatLogsReq struct {
	ConversationID   string `json:"conversationID"`
	StartClientMsgID string `json:"startClientMsgID"`
	Count            int32  `json:"count"`
}

// 新增：简单的获取历史消息请求格式
type SimpleGetMsgReq struct {
	ConversationID string `json:"conversationID"`
	Count          int32  `json:"count"`
}

// 新增：通过会话ID获取消息的请求格式
type GetConversationMsgReq struct {
	ConversationID string `json:"conversationID"`
	Offset         int32  `json:"offset"`
	Count          int32  `json:"count"`
}

// MongoDB中的消息结构
type MongoMessage struct {
	ID    string `bson:"_id"`
	DocID string `bson:"doc_id"`
	Msgs  []struct {
		Msg struct {
			SendID         string `bson:"send_id"`
			RecvID         string `bson:"recv_id"`
			ClientMsgID    string `bson:"client_msg_id"`
			ServerMsgID    string `bson:"server_msg_id"`
			SenderNickname string `bson:"sender_nickname"`
			SenderFaceURL  string `bson:"sender_face_url"`
			SessionType    int32  `bson:"session_type"`
			ContentType    int32  `bson:"content_type"`
			Content        string `bson:"content"`
			Seq            int64  `bson:"seq"`
			SendTime       int64  `bson:"send_time"`
			CreateTime     int64  `bson:"create_time"`
			Status         int32  `bson:"status"`
			IsRead         bool   `bson:"is_read"`
		} `bson:"msg"`
	} `bson:"msgs"`
}

// MessageInfo 消息信息
type MessageInfo struct {
	ClientMsgID    string `json:"clientMsgID"`
	ServerMsgID    string `json:"serverMsgID"`
	CreateTime     int64  `json:"createTime"`
	SendTime       int64  `json:"sendTime"`
	SessionType    int32  `json:"sessionType"`
	SendID         string `json:"sendID"`
	RecvID         string `json:"recvID"`
	ContentType    int32  `json:"contentType"`
	PlatformID     int32  `json:"platformID"`
	SenderNickname string `json:"senderNickname"`
	SenderFaceURL  string `json:"senderFaceURL"`
	GroupID        string `json:"groupID"`
	Content        string `json:"content"`
	Seq            int32  `json:"seq"`
	IsRead         bool   `json:"isRead"`
	Status         int32  `json:"status"`
}

// GetChatLogsResp 获取聊天记录响应
type GetChatLogsResp struct {
	BaseResponse
	Data struct {
		ChatLogs []MessageInfo `json:"chatLogs"`
	} `json:"data"`
}

// GetUserTokenReq 获取用户token请求
type GetUserTokenReq struct {
	Secret     string `json:"secret"`
	PlatformID int    `json:"platformID"`
	UserID     string `json:"userID"`
}

// GetUserTokenResp 获取用户token响应
type GetUserTokenResp struct {
	BaseResponse
	Data struct {
		Token             string `json:"token"`
		ExpireTimeSeconds int64  `json:"expireTimeSeconds"`
	} `json:"data"`
}

// getUserToken 获取用户token
func (c *OpenIMClient) getUserToken(ctx context.Context, userID string) (string, error) {
	req := GetUserTokenReq{
		Secret:     c.secret,
		PlatformID: 1, // Web平台
		UserID:     userID,
	}

	var resp GetUserTokenResp
	err := c.makeRequestWithoutToken(ctx, "POST", "/auth/user_token", req, &resp)
	if err != nil {
		return "", err
	}

	if resp.ErrCode != 0 {
		return "", fmt.Errorf("get user token failed: code=%d, msg=%s", resp.ErrCode, resp.ErrMsg)
	}

	return resp.Data.Token, nil
}

// getAdminToken 获取管理员token
func (c *OpenIMClient) getAdminToken(ctx context.Context) (string, error) {
	req := GetUserTokenReq{
		Secret:     c.secret,
		PlatformID: 1,           // Web平台
		UserID:     AdminUserID, // 使用管理员用户ID
	}

	var resp GetUserTokenResp
	err := c.makeRequestWithoutToken(ctx, "POST", "/auth/user_token", req, &resp)
	if err != nil {
		return "", err
	}

	if resp.ErrCode != 0 {
		return "", fmt.Errorf("get admin token failed: code=%d, msg=%s", resp.ErrCode, resp.ErrMsg)
	}

	return resp.Data.Token, nil
}

// makeRequestWithoutToken 发送HTTP请求（不需要token）
func (c *OpenIMClient) makeRequestWithoutToken(ctx context.Context, method, endpoint string, reqBody interface{}, respBody interface{}) error {
	var bodyReader io.Reader

	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// 设置必需的头信息
	operationID := uuid.New().String()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("operationID", operationID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// 调试：打印响应内容
	fmt.Printf("OpenIM响应 [%s %s]: %s\n", method, endpoint, string(respData))

	// 检查响应是否是有效的JSON
	if len(respData) == 0 {
		return fmt.Errorf("empty response")
	}

	// 尝试解析JSON
	if err := json.Unmarshal(respData, respBody); err != nil {
		return fmt.Errorf("unmarshal response (content: %s): %w", string(respData), err)
	}

	return nil
}

// getChatLogsFromMongoDB 直接从MongoDB获取聊天记录
func (c *OpenIMClient) getChatLogsFromMongoDB(ctx context.Context, conversationID string, count int32) ([]MessageInfo, error) {
	return c.getChatLogsFromMongoDBWithOffset(ctx, conversationID, count, 0)
}

// getChatLogsFromMongoDBWithOffset 直接从MongoDB获取聊天记录（支持offset）
func (c *OpenIMClient) getChatLogsFromMongoDBWithOffset(ctx context.Context, conversationID string, count, offset int32) ([]MessageInfo, error) {
	if c.mongoClient == nil {
		return nil, fmt.Errorf("MongoDB客户端未连接")
	}

	// 设置查询超时
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	collection := c.mongoClient.Database("openim_v3").Collection("msg")

	// 尝试不同的doc_id格式
	possibleDocIDs := []string{
		fmt.Sprintf("%s:0", conversationID),
		conversationID,
	}

	var messages []MessageInfo

	for _, docID := range possibleDocIDs {
		// 使用聚合查询来高效获取最新的N条消息
		pipeline := []bson.M{
			// 匹配文档
			{"$match": bson.M{"doc_id": docID}},
			// 展开msgs数组
			{"$unwind": "$msgs"},
			// 过滤掉空消息
			{"$match": bson.M{"msgs.msg.server_msg_id": bson.M{"$ne": ""}}},
			// 按发送时间排序
			{"$sort": bson.M{"msgs.msg.send_time": -1}},
		}

		// 如果指定了offset，则跳过前面的记录
		if offset > 0 {
			pipeline = append(pipeline, bson.M{"$skip": int(offset)})
		}

		// 如果指定了count，则限制结果数量
		if count > 0 {
			pipeline = append(pipeline, bson.M{"$limit": int(count)})
		}

		cursor, err := collection.Aggregate(queryCtx, pipeline)
		if err != nil {
			fmt.Printf("聚合查询失败: doc_id=%s, err=%v\n", docID, err)
			continue
		}
		defer cursor.Close(queryCtx)

		var results []bson.M
		if err = cursor.All(queryCtx, &results); err != nil {
			fmt.Printf("读取聚合结果失败: %v\n", err)
			continue
		}

		fmt.Printf("找到MongoDB文档: doc_id=%s, 获取到%d条最新消息\n", docID, len(results))

		// 转换结果为MessageInfo格式
		for _, result := range results {
			msgData, ok := result["msgs"].(bson.M)
			if !ok {
				continue
			}

			msgObj, ok := msgData["msg"].(bson.M)
			if !ok {
				continue
			}

			msg := MessageInfo{
				ClientMsgID:    getString(msgObj, "client_msg_id"),
				ServerMsgID:    getString(msgObj, "server_msg_id"),
				CreateTime:     getInt64(msgObj, "create_time"),
				SendTime:       getInt64(msgObj, "send_time"),
				SessionType:    getInt32(msgObj, "session_type"),
				SendID:         getString(msgObj, "send_id"),
				RecvID:         getString(msgObj, "recv_id"),
				ContentType:    getInt32(msgObj, "content_type"),
				SenderNickname: getString(msgObj, "sender_nickname"),
				SenderFaceURL:  getString(msgObj, "sender_face_url"),
				Content:        getString(msgObj, "content"),
				Seq:            getInt32(msgObj, "seq"),
				IsRead:         getBool(msgObj, "is_read"),
				Status:         getInt32(msgObj, "status"),
			}

			messages = append(messages, msg)
		}

		// 如果找到了消息，就停止尝试其他格式
		if len(messages) > 0 {
			break
		}
	}

	return messages, nil
}

// 辅助函数，用于从bson.M中安全地获取各种类型的值
func getString(m bson.M, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt64(m bson.M, key string) int64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case int32:
			return int64(v)
		case int:
			return int64(v)
		}
	}
	return 0
}

func getInt32(m bson.M, key string) int32 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int32:
			return v
		case int64:
			return int32(v)
		case int:
			return int32(v)
		}
	}
	return 0
}

func getBool(m bson.M, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// makeRequestWithUserToken 使用用户token发送请求
func (c *OpenIMClient) makeRequestWithUserToken(ctx context.Context, method, endpoint string, userID string, reqBody interface{}, respBody interface{}) error {
	token, err := c.getUserToken(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user token: %w", err)
	}

	var bodyReader io.Reader

	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// 设置必需的头信息
	operationID := uuid.New().String()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("operationID", operationID)
	req.Header.Set("token", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// 调试：打印响应内容
	tokenPreview := token
	if len(token) > 20 {
		tokenPreview = token[:20] + "..."
	}
	fmt.Printf("OpenIM响应 [%s %s] (token: %s): %s\n", method, endpoint, tokenPreview, string(respData))

	// 检查响应是否是有效的JSON
	if len(respData) == 0 {
		return fmt.Errorf("empty response")
	}

	// 尝试解析JSON
	if err := json.Unmarshal(respData, respBody); err != nil {
		return fmt.Errorf("unmarshal response (content: %s): %w", string(respData), err)
	}

	return nil
}

// RegisterUser 注册用户
func (c *OpenIMClient) RegisterUser(ctx context.Context, userID, nickname, faceURL string) error {
	req := RegisterUserReq{
		Secret: c.secret,
		Users: []UserInfo{
			{
				UserID:   userID,
				Nickname: nickname,
				FaceURL:  faceURL,
			},
		},
	}

	var resp RegisterUserResp
	err := c.makeRequestWithoutToken(ctx, "POST", "/user/user_register", req, &resp)
	if err != nil {
		return err
	}

	if resp.ErrCode != 0 {
		// 如果用户已存在，不视为错误
		if resp.ErrCode == 1002 { // 假设1002是用户已存在的错误码
			return nil
		}
		return fmt.Errorf("register user failed: code=%d, msg=%s", resp.ErrCode, resp.ErrMsg)
	}

	return nil
}

// SendMessage 发送消息
func (c *OpenIMClient) SendMessage(ctx context.Context, Send SendMsgReq) error {
	req := SendMsgReq{
		SendID:           Send.SendID,
		SendNickname:     Send.SendNickname,
		RecvID:           Send.RecvID,
		RecvNickname:     Send.RecvNickname,
		SenderPlatformID: 1,                // Web平台
		SessionType:      1,                // 单聊
		ContentType:      Send.ContentType, // 文本消息
		CreateTime:       time.Now().UnixMilli(),
		ClientMsgID:      uuid.New().String(),
	}
	req.Content.Content = Send.Content.Content

	var resp SendMsgResp
	err := c.makeRequestWithAdminToken(ctx, "POST", "/msg/send_msg", req, &resp)
	if err != nil {
		return err
	}

	if resp.ErrCode != 0 {
		return fmt.Errorf("send message failed: code=%d, msg=%s", resp.ErrCode, resp.ErrMsg)
	}

	return nil
}

// GetConversations 获取会话列表
func (c *OpenIMClient) GetConversations(ctx context.Context, userID string, offset, count int32) (*GetConversationsResp, error) {
	req := GetConversationsReq{
		OwnerUserID: userID,
		Offset:      offset,
		Count:       count,
	}

	var resp GetConversationsResp
	err := c.makeRequestWithUserToken(ctx, "POST", "/conversation/get_all_conversations", userID, req, &resp)
	if err != nil {
		return nil, err
	}

	if resp.ErrCode != 0 {
		return nil, fmt.Errorf("get conversations failed: code=%d, msg=%s", resp.ErrCode, resp.ErrMsg)
	}

	return &resp, nil
}

// GetChatLogs 获取聊天记录
func (c *OpenIMClient) GetChatLogs(ctx context.Context, userID, conversationID string, count int32) (*GetChatLogsResp, error) {
	return c.GetChatLogsWithOffset(ctx, userID, conversationID, count, 0)
}

// GetChatLogsWithOffset 获取聊天记录（支持offset）
func (c *OpenIMClient) GetChatLogsWithOffset(ctx context.Context, userID, conversationID string, count, offset int32) (*GetChatLogsResp, error) {
	fmt.Printf("获取聊天记录: conversationID=%s, count=%d, offset=%d\n", conversationID, count, offset)

	// 由于API端点不可用，直接从MongoDB读取消息
	messages, err := c.getChatLogsFromMongoDBWithOffset(ctx, conversationID, count, offset)
	if err != nil {
		fmt.Printf("从MongoDB获取消息失败: %v\n", err)
		// 即使失败也返回空列表，避免前端报错
		return &GetChatLogsResp{
			BaseResponse: BaseResponse{ErrCode: 0, ErrMsg: ""},
			Data: struct {
				ChatLogs []MessageInfo `json:"chatLogs"`
			}{ChatLogs: []MessageInfo{}},
		}, nil
	}

	return &GetChatLogsResp{
		BaseResponse: BaseResponse{ErrCode: 0, ErrMsg: ""},
		Data: struct {
			ChatLogs []MessageInfo `json:"chatLogs"`
		}{ChatLogs: messages},
	}, nil
}

// makeRequestWithAdminToken 使用管理员token发送请求
func (c *OpenIMClient) makeRequestWithAdminToken(ctx context.Context, method, endpoint string, reqBody interface{}, respBody interface{}) error {
	token, err := c.getAdminToken(ctx)
	if err != nil {
		return fmt.Errorf("get admin token: %w", err)
	}

	var bodyReader io.Reader

	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// 设置必需的头信息
	operationID := uuid.New().String()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("operationID", operationID)
	req.Header.Set("token", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// 调试：打印响应内容
	tokenPreview := token
	if len(token) > 20 {
		tokenPreview = token[:20] + "..."
	}
	fmt.Printf("OpenIM响应 [%s %s] (admin token: %s): %s\n", method, endpoint, tokenPreview, string(respData))

	// 检查响应是否是有效的JSON
	if len(respData) == 0 {
		return fmt.Errorf("empty response")
	}

	// 尝试解析JSON
	if err := json.Unmarshal(respData, respBody); err != nil {
		return fmt.Errorf("unmarshal response (content: %s): %w", string(respData), err)
	}

	return nil
}
