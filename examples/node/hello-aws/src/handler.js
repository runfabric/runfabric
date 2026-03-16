exports.hello = async () => {
    return {
        statusCode: 200,
        body: JSON.stringify({ message: "hello route" }),
    };
};

exports.createUser = async (event) => {
    return {
        statusCode: 201,
        body: JSON.stringify({
            message: "user created",
            event: event || null,
        }),
    };
};